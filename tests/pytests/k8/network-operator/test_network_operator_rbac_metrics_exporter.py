
import os
import yaml
import pytest
import logging
from concurrent.futures import ThreadPoolExecutor, as_completed
from kubernetes import client as k8s_client
from util import *


NC_NAMESPACE = "kube-amd-network"
NETWORKCONFIG_NAME = os.environ.get("NETWORKCONFIG_NAME", "test-networkconfig")
METRICS_SERVICE_NAME = "my-metrics-service"
TARGET_PORT = 2001  # custom port to add under metricsExporter.port and to use for pulls



@pytest.mark.timeout(TEST_TIMEOUT)
def test_rbac_node_port():
    """
    Workflow:
    1) Patch relevant NetworkConfig objects (vf-* and non-vf- as applicable) to set
       spec.metricsExporter.rbacConfig.enable = True and ensure clientCAConfigMap.name = "client-ca".
    2) Trigger IB traffic on workload pods (this generates the traffic/metrics).
    3) After traffic, for each workload pod, choose the appropriate NetworkConfig (vf- vs non-vf-),
       read that config's spec.metricsExporter.nodePort, determine the pod's node InternalIP and
       pull metrics from https://<node_ip>:<nodePort>/metrics using local curl with mTLS
       (curl_metrics_from_local which uses LOCAL_CERT_DIR/client.{crt,key} and ca.crt).
    4) Validate Prometheus numeric lines in responses.
    5) Restore all modified NetworkConfig objects to their originals.
    """
    load_kube_config()
    v1 = k8s_client.CoreV1Api()
    co_api = k8s_client.CustomObjectsApi()

    # 1) List NetworkConfig CRs and save originals
    try:
        nc_items = list_networkconfigs_custom(NC_NAMESPACE)
    except Exception as e:
        pytest.skip(f"Could not list NetworkConfig resources in {NC_NAMESPACE}: {e}")

    if not nc_items:
        pytest.skip(f"No NetworkConfig resources in {NC_NAMESPACE}")

    originals = {}
    for it in nc_items:
        name = it.get("metadata", {}).get("name")
        if not name:
            continue
        originals[name] = yaml.safe_load(yaml.safe_dump(it))

    # Build patch to enable rbac under spec.metricsExporter for all NetworkConfigs
    patch_body = {
        "spec": {
            "metricsExporter": {
                "rbacConfig": {
                    "clientCAConfigMap": {"name": "client-ca"},
                    "enable": True
                }
            }
        }
    }

    # Apply patches in parallel
    applied = []
    failed = []
    with ThreadPoolExecutor(max_workers=min(MAX_WORKERS, max(2, len(originals)))) as ex:
        futures = {ex.submit(patch_networkconfig_custom, NC_NAMESPACE, name, patch_body): name for name in originals.keys()}
        for fut in as_completed(futures):
            name = futures[fut]
            try:
                fut.result()
                applied.append(name)
                LOG.info("Patched %s -> spec.metricsExporter.rbacConfig.enable=True", name)
            except Exception as e:
                LOG.error("Failed to patch %s: %s", name, e)
                failed.append(name)

    if failed:
        # rollback applied ones and fail early
        for rn in applied:
            try:
                replace_with_retry(NC_NAMESPACE, rn, originals[rn])
            except Exception as re:
                LOG.error("Rollback failed for %s: %s", rn, re)
        pytest.fail(f"Failed to apply RBAC enable patch to some NetworkConfigs: {failed}")

    # Optional: give operator a short moment to reconcile (increase if necessary)
    time.sleep(2)

    # 2) Trigger IB traffic on workload pods (best-effort)
    wpods = list_workloads(v1, namespace="default")
    if not wpods:
        # revert before skipping
        for rn, orig in originals.items():
            try:
                replace_with_retry(NC_NAMESPACE, rn, orig)
            except Exception:
                LOG.error("Failed revert for %s during skip", rn)
        pytest.skip("No running workload pods in default namespace")

    exec_outputs = {}
    with ThreadPoolExecutor(max_workers=min(MAX_WORKERS, max(2, len(wpods)))) as ex:
        futures = {ex.submit(run_ib_traffic, v1, p.metadata.name, p.metadata.namespace): p for p in wpods}
        for fut in as_completed(futures):
            p = futures[fut]
            try:
                out = fut.result()
                exec_outputs.update(out)
            except Exception as e:
                LOG.warning("run_ib_traffic failed for %s: %s", p.metadata.name, e)
                exec_outputs[p.metadata.name] = f"ERROR: {e}"

    # 3) After traffic, build map of NetworkConfig -> nodePort
    nodeport_by_config = {}
    for it in list_networkconfigs_custom(NC_NAMESPACE):
        name = it.get("metadata", {}).get("name")
        if not name:
            continue
        try:
            np = get_metrics_nodeport_from_networkconfig(co_api, NC_NAMESPACE, name)
        except Exception:
            np = None
        nodeport_by_config[name] = np

    vf_configs = [n for n in nodeport_by_config.keys() if n.startswith("vf-")]
    non_vf_configs = [n for n in nodeport_by_config.keys() if not n.startswith("vf-")]

    # 4) For each workload pod select corresponding config and pull metrics using the nodePort
    metrics_found = {}
    metrics_missing = []

    for p in wpods:
        pod_name = p.metadata.name
        node_name = getattr(p.spec, "node_name", None) or getattr(p.spec, "nodeName", None)
        if not node_name:
            LOG.warning("Pod %s has no nodeName; skipping", pod_name)
            metrics_missing.append((pod_name, "no-nodeName"))
            continue

        node_ip = get_node_ip(v1, node_name)
        if not node_ip:
            LOG.warning("No InternalIP for node %s (pod %s)", node_name, pod_name)
            metrics_missing.append((pod_name, f"no-node-ip node={node_name}"))
            continue

        # choose config by vf- prefix rule
        if pod_name.startswith("vf-workload"):
            cfg = vf_configs[0] if vf_configs else None
        else:
            cfg = non_vf_configs[0] if non_vf_configs else None

        if not cfg:
            LOG.error("No matching NetworkConfig for pod %s", pod_name)
            metrics_missing.append((pod_name, "no-matching-config"))
            continue

        port = nodeport_by_config.get(cfg)
        if not port:
            LOG.warning("No numeric nodePort for config %s; skipping pod %s", cfg, pod_name)
            metrics_missing.append((pod_name, f"no-nodePort-for-config {cfg}"))
            continue

        LOG.info("Pulling metrics for pod %s -> node %s (%s) port %s (config=%s)", pod_name, node_name, node_ip, port, cfg)

        # try a few times (small retries) to account for timing
        txt = None
        ok = False
        for attempt in range(3):
            txt = curl_metrics_from_local(node_ip, port, METRICS_SERVICE_NAME, timeout=8)
            if txt and txt.strip():
                # check for prometheus numeric line
                for ln in txt.splitlines():
                    ln = ln.strip()
                    if not ln or ln.startswith("#"):
                        continue
                    if PROM_LINE_RE.match(ln):
                        ok = True
                        break
            if ok:
                break
            time.sleep(1)

        if ok:
            metrics_found[pod_name] = {"node": node_name, "node_ip": node_ip, "port": port}
            LOG.info("Metrics OK for pod %s on %s:%s", pod_name, node_ip, port)
        else:
            sample = (txt or "")[:1000]
            LOG.warning("Failed to fetch valid metrics for pod %s from %s:%s sample=%s", pod_name, node_ip, port, sample)
            metrics_missing.append((pod_name, f"no-metrics node_ip={node_ip} port={port} sample={sample}"))

    # 5) Revert all modified NetworkConfig objects
    revert_errors = []
    for rn, orig in originals.items():
        try:
            replace_with_retry(NC_NAMESPACE, rn, orig)
            LOG.info("Restored original NetworkConfig %s", rn)
        except Exception as e:
            LOG.error("Failed to restore %s: %s", rn, e)
            revert_errors.append((rn, str(e)))

    if revert_errors:
        LOG.error("One or more NetworkConfig originals failed to revert: %s", revert_errors)
        pytest.fail(f"Failed to revert original NetworkConfig(s): {revert_errors}")

    if metrics_missing:
        LOG.error("Metrics missing/invalid after traffic: %s", metrics_missing)
        pytest.fail(f"Metrics missing/invalid after traffic: {metrics_missing}")

    LOG.info("Successfully validated metrics after enabling RBAC and running IB traffic for pods: %s", list(metrics_found.keys()))

