#!/usr/bin/env python3
import time
import yaml
import pytest
import logging
from typing import Dict, Any
from concurrent.futures import ThreadPoolExecutor, as_completed

from kubernetes import client as k8s_client

# import helpers from util.py
from util import *

# ---------- Tests ----------
@pytest.mark.timeout(TEST_TIMEOUT)
def test_all_pods_running():
    load_kube_config()
    v1 = k8s_client.CoreV1Api()
    pods = list_pods(v1)
    for p in pods:
        if p.status.phase != "Running":
            LOG.error("Not Ok pod=%s", p.metadata.name)
    LOG.info("Checked pod phases.")


@pytest.mark.timeout(TEST_TIMEOUT)
def test_ib_traffic_and_pull_metrics():
    load_kube_config()
    v1 = k8s_client.CoreV1Api()

    nc_namespace = "kube-amd-network"
    try:
        nc_items = list_networkconfigs_custom(nc_namespace)
    except Exception as e:
        pytest.skip(f"Could not list NetworkConfig resources: {e}")

    nodeport_by_config: Dict[str, int] = {}
    for it in nc_items:
        name = it.get("metadata", {}).get("name")
        if not name:
            continue
        node_port = it.get("spec", {}).get("metricsExporter", {}).get("nodePort")
        if node_port is None:
            pytest.fail(f"{name} missing spec.metricsExporter.nodePort")
        nodeport_by_config[name] = int(node_port)

    vf_configs = [n for n in nodeport_by_config if n.startswith("vf-")]
    non_vf_configs = [n for n in nodeport_by_config if not n.startswith("vf-")]

    all_pods = list_pods(v1, "default")
    wpods = [p for p in all_pods if p.status.phase == "Running"]
    if not wpods:
        pytest.skip("No running workload pods in default")

    exec_outputs = {}
    with ThreadPoolExecutor(max_workers=min(MAX_WORKERS, max(2, len(wpods)))) as ex:
        futures = {ex.submit(run_ib_traffic, v1, p.metadata.name, p.metadata.namespace): p for p in wpods}
        for fut in as_completed(futures):
            p = futures[fut]
            try:
                out = fut.result()
                exec_outputs.update(out)
            except Exception as e:
                LOG.error("run_ib_traffic failed for %s: %s", p.metadata.name, e)
                exec_outputs[p.metadata.name] = f"ERROR: {e}"

    pull_inputs = []
    for p in wpods:
        pod_name = p.metadata.name
        node_name = getattr(p.spec, "node_name", None) or getattr(p.spec, "nodeName", None)
        node_ip = get_node_ip(v1, node_name)
        if pod_name.startswith("vf-workload"):
            cfg = vf_configs[0] if vf_configs else None
        else:
            cfg = non_vf_configs[0] if non_vf_configs else None
        if not cfg:
            pytest.fail(f"No NetworkConfig matching rule for pod {pod_name}")
        pull_inputs.append((p, node_ip, nodeport_by_config[cfg]))

    metrics_by_pod = {}
    metrics_missing = []
    with ThreadPoolExecutor(max_workers=min(MAX_WORKERS, max(2, len(pull_inputs)))) as ex:
        futures = {ex.submit(pull_metrics, v1, p.metadata.name, p.metadata.namespace, port, node_ip): (p, node_ip, port)
                   for (p, node_ip, port) in pull_inputs}
        for fut in as_completed(futures):
            p, node_ip, port = futures[fut]
            pod_name = p.metadata.name
            try:
                txt = fut.result()
            except Exception as e:
                LOG.error("pull_metrics failed for %s: %s", pod_name, e)
                txt = None
            if not txt or not txt.strip():
                metrics_missing.append((pod_name, f"node_ip={node_ip} port={port}"))
                continue
            found = False
            for ln in txt.splitlines():
                ln = ln.strip()
                if not ln or ln.startswith("#"):
                    continue
                from util import PROM_LINE_RE  # local import to access regex
                if PROM_LINE_RE.match(ln):
                    found = True
                    break
            if not found:
                metrics_missing.append((pod_name, f"no numeric lines from {node_ip}:{port}"))
            else:
                metrics_by_pod[pod_name] = txt

    if metrics_missing:
        LOG.error("Metrics missing/invalid: %s", metrics_missing)
        pytest.fail(f"Metrics missing/invalid: {metrics_missing}")

    LOG.info("Metrics validated for pods: %s", list(metrics_by_pod.keys()))


@pytest.mark.timeout(TEST_TIMEOUT)
def test_update_nodeport_and_verify_metrics_pull():
    load_kube_config()
    v1 = k8s_client.CoreV1Api()
    nc_namespace = "kube-amd-network"

    try:
        items = list_networkconfigs_custom(nc_namespace)
    except Exception as e:
        pytest.skip(f"Could not list NetworkConfig resources: {e}")

    if not items:
        pytest.skip("No NetworkConfig objects found")

    originals = {}
    modified_vals = {}
    for it in items:
        name = it.get("metadata", {}).get("name")
        if not name:
            continue
        originals[name] = yaml.safe_load(yaml.safe_dump(it))
        modified_vals[name] = 32520 if name.startswith("vf-") else 32521

    applied = []
    with ThreadPoolExecutor(max_workers=min(MAX_WORKERS, max(2, len(modified_vals)))) as ex:
        futures = {ex.submit(patch_networkconfig_custom, nc_namespace, name, {"spec": {"metricsExporter": {"nodePort": port}}}): name
                   for name, port in modified_vals.items()}
        for fut in as_completed(futures):
            name = futures[fut]
            try:
                fut.result()
                applied.append(name)
                LOG.info("Patched %s -> nodePort=%d", name, modified_vals[name])
            except Exception as e:
                LOG.error("Failed to patch %s: %s", name, e)
                for rn in applied:
                    try:
                        orig_port = originals[rn]["spec"]["metricsExporter"].get("nodePort")
                        patch_networkconfig_custom(nc_namespace, rn, {"spec": {"metricsExporter": {"nodePort": orig_port}}})
                    except Exception as re:
                        LOG.error("Rollback failed for %s: %s", rn, re)
                pytest.fail(f"Patch apply failed: {e}")

    all_pods = list_pods(v1, "default")
    wpods = [p for p in all_pods if p.status.phase == "Running"]
    if not wpods:
        for rn, orig in originals.items():
            try:
                replace_with_retry(nc_namespace, rn, orig)
            except Exception:
                LOG.error("Failed restore during skip")
        pytest.skip("No running workload pods")

    reps = {}
    for p in wpods:
        pod_name = p.metadata.name
        cfg = next((n for n in modified_vals.keys() if (n.startswith("vf-") if pod_name.startswith("vf-workload") else not n.startswith("vf-"))), None)
        if cfg and cfg not in reps:
            reps[cfg] = p

    for cfg, rep_pod in reps.items():
        port = modified_vals[cfg]
        node_name = getattr(rep_pod.spec, "node_name", None) or getattr(rep_pod.spec, "nodeName", None)
        node_ip = get_node_ip(v1, node_name)
        from util import wait_for_metrics_ready
        ok = wait_for_metrics_ready(v1, rep_pod.metadata.name, rep_pod.metadata.namespace, node_ip, port, timeout=20, interval=1.0)
        if not ok:
            LOG.warning("Metrics not ready for config %s on %s:%d", cfg, node_ip, port)

    with ThreadPoolExecutor(max_workers=min(MAX_WORKERS, max(2, len(wpods)))) as ex:
        futures = {ex.submit(run_ib_traffic, v1, p.metadata.name, p.metadata.namespace): p for p in wpods}
        for fut in as_completed(futures):
            _ = fut.result()

    pull_inputs = []
    for p in wpods:
        pod_name = p.metadata.name
        cfg = next((n for n in modified_vals.keys() if (n.startswith("vf-") if pod_name.startswith("vf-workload") else not n.startswith("vf-"))), None)
        if not cfg:
            continue
        node_name = getattr(p.spec, "node_name", None) or getattr(p.spec, "nodeName", None)
        node_ip = get_node_ip(v1, node_name)
        pull_inputs.append((p, node_ip, modified_vals[cfg]))

    metrics_missing = []
    metrics_found = {}
    with ThreadPoolExecutor(max_workers=min(MAX_WORKERS, max(2, len(pull_inputs)))) as ex:
        futures = {ex.submit(pull_metrics, v1, p.metadata.name, p.metadata.namespace, port, node_ip): (p, node_ip, port)
                   for (p, node_ip, port) in pull_inputs}
        for fut in as_completed(futures):
            p, node_ip, port = futures[fut]
            pod_name = p.metadata.name
            txt = fut.result()
            if not txt or not txt.strip():
                metrics_missing.append((pod_name, f"node_ip={node_ip} port={port}"))
                continue
            found = False
            for ln in txt.splitlines():
                ln = ln.strip()
                if not ln or ln.startswith("#"):
                    continue
                from util import PROM_LINE_RE
                if PROM_LINE_RE.match(ln):
                    found = True
                    break
            if not found:
                metrics_missing.append((pod_name, f"no numeric lines from {node_ip}:{port}"))
            else:
                metrics_found[pod_name] = txt

    revert_errors = []
    for rn, orig in originals.items():
        try:
            replace_with_retry(nc_namespace, rn, orig)
        except Exception as e:
            LOG.error("Failed to revert %s: %s", rn, e)
            revert_errors.append((rn, str(e)))

    if revert_errors:
        LOG.error("Revert errors: %s", revert_errors)

    if metrics_missing:
        LOG.error("Metrics missing after nodePort change: %s", metrics_missing)
        pytest.fail(f"Metrics missing after nodePort change: {metrics_missing}")

    LOG.info("Metrics OK for updated nodePorts on pods: %s", list(metrics_found.keys()))


@pytest.mark.timeout(TEST_TIMEOUT)
def test_disable_metrics_exporter_and_verify_no_metrics():
    load_kube_config()
    v1 = k8s_client.CoreV1Api()
    nc_namespace = "kube-amd-network"

    try:
        items = list_networkconfigs_custom(nc_namespace)
    except Exception as e:
        pytest.skip(f"Could not list NetworkConfig resources: {e}")

    if not items:
        pytest.skip("No NetworkConfig objects found")

    originals = {}
    nodeport_by_config = {}
    for it in items:
        name = it.get("metadata", {}).get("name")
        if not name:
            continue
        originals[name] = yaml.safe_load(yaml.safe_dump(it))
        node_port = it.get("spec", {}).get("metricsExporter", {}).get("nodePort")
        if node_port is None:
            pytest.fail(f"{name} missing spec.metricsExporter.nodePort")
        nodeport_by_config[name] = int(node_port)

    with ThreadPoolExecutor(max_workers=min(MAX_WORKERS, max(2, len(originals)))) as ex:
        futures = {ex.submit(patch_networkconfig_custom, nc_namespace, name, {"spec": {"metricsExporter": {"enable": False}}}): name
                   for name in originals.keys()}
        failed = []
        for fut in as_completed(futures):
            name = futures[fut]
            try:
                fut.result()
            except Exception as e:
                LOG.error("Failed to patch disable for %s: %s", name, e)
                failed.append(name)
        if failed:
            for rn in originals.keys():
                try:
                    replace_with_retry(nc_namespace, rn, originals[rn])
                except Exception:
                    LOG.error("Rollback failed during disable error path")
            pytest.fail(f"Failed to disable some NetworkConfigs: {failed}")

    time.sleep(2)

    all_pods = list_pods(v1, "default")
    wpods = [p for p in all_pods if p.status.phase == "Running"]
    if not wpods:
        for rn, orig in originals.items():
            try:
                replace_with_retry(nc_namespace, rn, orig)
            except Exception:
                LOG.error("Restore failed during skip")
        pytest.skip("No running workload pods")

    with ThreadPoolExecutor(max_workers=min(MAX_WORKERS, max(2, len(wpods)))) as ex:
        futures = {ex.submit(run_ib_traffic, v1, p.metadata.name, p.metadata.namespace): p for p in wpods}
        for fut in as_completed(futures):
            _ = fut.result()

    vf_configs = [n for n in nodeport_by_config if n.startswith("vf-")]
    non_vf_configs = [n for n in nodeport_by_config if not n.startswith("vf-")]
    pull_inputs = []
    for p in wpods:
        pod_name = p.metadata.name
        node_name = getattr(p.spec, "node_name", None) or getattr(p.spec, "nodeName", None)
        node_ip = get_node_ip(v1, node_name)
        cfg = vf_configs[0] if pod_name.startswith("vf-workload") else (non_vf_configs[0] if non_vf_configs else None)
        if not cfg:
            for rn, orig in originals.items():
                try:
                    replace_with_retry(nc_namespace, rn, orig)
                except Exception:
                    LOG.error("Restore failed")
            pytest.fail(f"No NetworkConfig matching rule for pod {pod_name}")
        pull_inputs.append((p, node_ip, nodeport_by_config[cfg]))

    metrics_found = {}
    with ThreadPoolExecutor(max_workers=min(MAX_WORKERS, max(2, len(pull_inputs)))) as ex:
        futures = {ex.submit(pull_metrics, v1, p.metadata.name, p.metadata.namespace, port, node_ip): (p, node_ip, port)
                   for (p, node_ip, port) in pull_inputs}
        for fut in as_completed(futures):
            p, node_ip, port = futures[fut]
            pod_name = p.metadata.name
            txt = fut.result()
            has_numeric = False
            if txt and txt.strip():
                for ln in txt.splitlines():
                    ln = ln.strip()
                    if not ln or ln.startswith("#"):
                        continue
                    from util import PROM_LINE_RE
                    if PROM_LINE_RE.match(ln):
                        has_numeric = True
                        break
            if has_numeric:
                metrics_found[pod_name] = {"node_ip": node_ip, "port": port}

    for rn, orig in originals.items():
        try:
            replace_with_retry(nc_namespace, rn, orig)
        except Exception as e:
            LOG.error("Failed to restore %s: %s", rn, e)

    if metrics_found:
        LOG.error("Expected no metrics, but found metrics for pods: %s", metrics_found)
        pytest.fail(f"Metrics were returned despite disabling exporter: {list(metrics_found.keys())}")

    LOG.info("Negative test passed: no numeric metrics found after disabling exporters.")


@pytest.mark.timeout(TEST_TIMEOUT)
def test_out_of_range_nodeport():
    load_kube_config()
    v1 = k8s_client.CoreV1Api()
    nc_namespace = "kube-amd-network"
    OUT_OF_RANGE_PORT = 32800  # outside Kubernetes NodePort default 30000-32767

    try:
        items = list_networkconfigs_custom(nc_namespace)
    except Exception as e:
        pytest.skip(f"Could not list NetworkConfig resources: {e}")

    if not items:
        pytest.skip("No NetworkConfig objects found in namespace")

    originals: Dict[str, Dict[str, Any]] = {}
    names = []
    for it in items:
        name = it.get("metadata", {}).get("name")
        if not name:
            continue
        originals[name] = yaml.safe_load(yaml.safe_dump(it))
        names.append(name)

    succeeded = []
    failed = []

    for name in names:
        patch_body = {"spec": {"metricsExporter": {"nodePort": OUT_OF_RANGE_PORT}}}
        try:
            patch_networkconfig_custom(nc_namespace, name, patch_body)
            LOG.warning("Patch unexpectedly succeeded for %s with nodePort=%d", name, OUT_OF_RANGE_PORT)
            succeeded.append(name)
        except ApiException as e:
            LOG.info("Patch rejected for %s as expected: status=%s reason=%s", name, getattr(e, "status", None), getattr(e, "reason", None))
            failed.append((name, getattr(e, "status", None), getattr(e, "body", None)))
        except Exception as e:
            LOG.info("Patch raised exception for %s (treated as rejection): %s", name, e)
            failed.append((name, "exception", str(e)))

    if succeeded:
        restore_errors = []
        for rn in succeeded:
            try:
                replace_with_retry(nc_namespace, rn, originals[rn])
            except Exception as e:
                LOG.error("Failed to restore original for %s after unexpected patch success: %s", rn, e)
                restore_errors.append((rn, str(e)))
        if restore_errors:
            LOG.error("Restore errors after unexpected acceptance: %s", restore_errors)
        pytest.fail(f"API unexpectedly accepted out-of-range nodePort for NetworkConfig(s): {succeeded}")

    revert_errors = []
    for rn, orig in originals.items():
        try:
            try:
                current = get_networkconfig_custom(nc_namespace, rn)
                o = yaml.safe_load(yaml.safe_dump(orig))
                o.setdefault("metadata", {})["resourceVersion"] = current.get("metadata", {}).get("resourceVersion")
                replace_with_retry(nc_namespace, rn, o)
            except Exception:
                replace_with_retry(nc_namespace, rn, orig)
        except Exception as e:
            LOG.error("Failed to restore original NetworkConfig %s: %s", rn, e)
            revert_errors.append((rn, str(e)))

    if revert_errors:
        LOG.error("One or more NetworkConfig originals failed to revert: %s", revert_errors)
        pytest.fail(f"Failed to revert original NetworkConfig(s): {revert_errors}")

    LOG.info("Setting out-of-range nodePort was correctly rejected for all NetworkConfig objects.")


@pytest.mark.timeout(TEST_TIMEOUT)
def test_pull_metrics_using_source_port():
    load_kube_config()
    v1 = k8s_client.CoreV1Api()
    nc_namespace = "kube-amd-network"
    TARGET_PORT = 5001

    try:
        items = list_networkconfigs_custom(nc_namespace)
    except Exception as e:
        pytest.skip(f"Could not list NetworkConfig resources: {e}")

    if not items:
        pytest.skip("No NetworkConfig objects found in namespace kube-amd-network")

    originals: Dict[str, Dict[str, Any]] = {}
    ip_val_by_config: Dict[str, str] = {}

    for it in items:
        name = it.get("metadata", {}).get("name")
        if not name:
            continue
        originals[name] = yaml.safe_load(yaml.safe_dump(it))
        raw_np = it.get("spec", {}).get("metricsExporter", {}).get("nodePort")
        if raw_np is None:
            ip_val_by_config[name] = ""
        else:
            ip_val_by_config[name] = str(raw_np)

    patched = []
    with ThreadPoolExecutor(max_workers=min(MAX_WORKERS, max(2, len(originals)))) as ex:
        futures = {
            ex.submit(patch_networkconfig_custom, nc_namespace, name, {"spec": {"serviceType": "ClusterIP"}}): name
            for name in originals.keys()
        }
        failed = []
        for fut in as_completed(futures):
            name = futures[fut]
            try:
                fut.result()
                patched.append(name)
                LOG.info("Patched %s to serviceType=ClusterIP", name)
            except Exception as e:
                LOG.error("Failed to patch serviceType for %s: %s", name, e)
                failed.append(name)
        if failed:
            for rn in originals.keys():
                try:
                    replace_with_retry(nc_namespace, rn, originals[rn])
                except Exception:
                    LOG.error("Rollback failed during serviceType patch error path")
            pytest.fail(f"Failed to set serviceType=ClusterIP for some NetworkConfigs: {failed}")

    wpods = list_workloads(v1, namespace="default")
    if not wpods:
        for rn, orig in originals.items():
            try:
                replace_with_retry(nc_namespace, rn, orig)
            except Exception:
                LOG.error("Failed to restore during skip")
        pytest.skip("No running workload pods in default namespace")

    with ThreadPoolExecutor(max_workers=min(MAX_WORKERS, max(2, len(wpods)))) as ex:
        futures = {ex.submit(run_ib_traffic, v1, p.metadata.name, p.metadata.namespace): p for p in wpods}
        for fut in as_completed(futures):
            _ = fut.result()

    metrics_unexpected = []
    metrics_empties = []

    for p in wpods:
        pod_name = p.metadata.name
        cfg = None
        if pod_name.startswith("vf-workload"):
            cfg = next((n for n in ip_val_by_config.keys() if n.startswith("vf-")), None)
        else:
            cfg = next((n for n in ip_val_by_config.keys() if not n.startswith("vf-")), None)

        if not cfg:
            LOG.warning("No NetworkConfig mapping found for pod %s; skipping", pod_name)
            continue

        candidate_ip = ip_val_by_config.get(cfg, "")
        if not candidate_ip:
            LOG.info("NetworkConfig %s has no nodePort value to test as IP; skipping pod %s", cfg, pod_name)
            continue

        # Retry curl with backoff to handle timing issues
        txt = None
        max_retries = 5
        retry_delay = 2
        for attempt in range(1, max_retries + 1):
            curl_cmd = f"curl -sS --connect-timeout 3 http://{candidate_ip}:{TARGET_PORT}/metrics || true"
            try:
                txt = exec_in_pod_sync(v1, pod_name, p.metadata.namespace, curl_cmd, timeout=5)
                if txt and txt.strip():
                    break
                LOG.debug("Attempt %d/%d: Empty response from %s:%d", attempt, max_retries, candidate_ip, TARGET_PORT)
            except Exception as e:
                LOG.debug("Attempt %d/%d: Failed to exec curl from pod %s for candidate '%s': %s", attempt, max_retries, pod_name, candidate_ip, e)
            if attempt < max_retries:
                time.sleep(retry_delay)

        if not txt or not txt.strip():
            metrics_empties.append((pod_name, candidate_ip))
            continue

        found_numeric = False
        for ln in txt.splitlines():
            ln = ln.strip()
            if not ln or ln.startswith("#"):
                continue
            from util import PROM_LINE_RE
            if PROM_LINE_RE.match(ln):
                found_numeric = True
                break

        if found_numeric:
            metrics_unexpected.append((pod_name, candidate_ip))
        else:
            metrics_empties.append((pod_name, candidate_ip))

    revert_errors = []
    for rn, orig in originals.items():
        try:
            replace_with_retry(nc_namespace, rn, orig)
        except Exception as e:
            LOG.error("Failed to restore %s: %s", rn, e)
            revert_errors.append((rn, str(e)))

    if revert_errors:
        LOG.error("Failed to revert some NetworkConfigs: %s", revert_errors)

    if metrics_unexpected:
        LOG.error("Unexpectedly succeeded fetching numeric metrics from IPs under nodePort: %s", metrics_unexpected)
        pytest.fail(f"Unexpectedly succeeded fetching numeric metrics from IPs under nodePort: {metrics_unexpected}")

    if not metrics_empties:
        pytest.skip("No candidate IPs found under spec.metricsExporter.nodePort to test; nothing to assert")

    LOG.info("Verified that IPs stored under nodePort did not yield metrics when serviceType=ClusterIP: %s", metrics_empties)


@pytest.mark.timeout(TEST_TIMEOUT)
def test_custom_source_port():
    load_kube_config()
    v1 = k8s_client.CoreV1Api()
    nc_namespace = "kube-amd-network"
    TARGET_PORT = 2001

    try:
        items = list_networkconfigs_custom(nc_namespace)
    except Exception as e:
        pytest.skip(f"Could not list NetworkConfig resources: {e}")

    if not items:
        pytest.skip("No NetworkConfig objects found in namespace kube-amd-network")

    originals: Dict[str, Dict[str, Any]] = {}
    nodeport_raw_by_config: Dict[str, str] = {}
    for it in items:
        name = it.get("metadata", {}).get("name")
        if not name:
            continue
        originals[name] = yaml.safe_load(yaml.safe_dump(it))
        raw_np = it.get("spec", {}).get("metricsExporter", {}).get("nodePort")
        nodeport_raw_by_config[name] = "" if raw_np is None else str(raw_np)

    patched = []
    with ThreadPoolExecutor(max_workers=min(MAX_WORKERS, max(2, len(originals)))) as ex:
        futures = {
            ex.submit(
                patch_networkconfig_custom,
                nc_namespace,
                name,
                {"spec": {"serviceType": "ClusterIP", "metricsExporter": {"port": TARGET_PORT}}},
            ): name
            for name in originals.keys()
        }
        failed = []
        for fut in as_completed(futures):
            name = futures[fut]
            try:
                fut.result()
                patched.append(name)
                LOG.info("Patched %s to serviceType=ClusterIP and metricsExporter.port=%d", name, TARGET_PORT)
            except Exception as e:
                LOG.error("Failed to patch %s: %s", name, e)
                failed.append(name)
        if failed:
            for rn in originals.keys():
                try:
                    replace_with_retry(nc_namespace, rn, originals[rn])
                except Exception:
                    LOG.error("Rollback failed during serviceType patch error path")
            pytest.fail(f"Failed to set serviceType=ClusterIP for some NetworkConfigs: {failed}")

    wpods = list_workloads(v1, namespace="default")
    if not wpods:
        for rn, orig in originals.items():
            try:
                replace_with_retry(nc_namespace, rn, orig)
            except Exception:
                LOG.error("Failed to restore during skip")
        pytest.skip("No running workload pods in default namespace")

    with ThreadPoolExecutor(max_workers=min(MAX_WORKERS, max(2, len(wpods)))) as ex:
        futures = {ex.submit(run_ib_traffic, v1, p.metadata.name, p.metadata.namespace): p for p in wpods}
        for fut in as_completed(futures):
            _ = fut.result()

    metrics_unexpected = []
    metrics_empties = []
    for p in wpods:
        pod_name = p.metadata.name
        if pod_name.startswith("vf-workload"):
            cfg = next((n for n in nodeport_raw_by_config.keys() if n.startswith("vf-")), None)
        else:
            cfg = next((n for n in nodeport_raw_by_config.keys() if not n.startswith("vf-")), None)
        if not cfg:
            LOG.warning("No NetworkConfig mapping found for pod %s; skipping", pod_name)
            continue
        candidate_ip = nodeport_raw_by_config.get(cfg, "")
        if not candidate_ip:
            LOG.info("NetworkConfig %s has no nodePort value to test as IP; skipping pod %s", cfg, pod_name)
            continue

        # Retry curl with backoff to handle timing issues
        txt = None
        max_retries = 5
        retry_delay = 2
        for attempt in range(1, max_retries + 1):
            curl_cmd = f"curl -sS --connect-timeout 3 http://{candidate_ip}:{TARGET_PORT}/metrics || true"
            try:
                txt = exec_in_pod_sync(v1, pod_name, p.metadata.namespace, curl_cmd, timeout=5)
                if txt and txt.strip():
                    break
                LOG.debug("Attempt %d/%d: Empty response from %s:%d", attempt, max_retries, candidate_ip, TARGET_PORT)
            except Exception as e:
                LOG.debug("Attempt %d/%d: Failed to exec curl from pod %s for candidate '%s': %s", attempt, max_retries, pod_name, candidate_ip, e)
            if attempt < max_retries:
                time.sleep(retry_delay)

        if not txt or not txt.strip():
            metrics_empties.append((pod_name, candidate_ip))
            continue

        found_numeric = False
        for ln in txt.splitlines():
            ln = ln.strip()
            if not ln or ln.startswith("#"):
                continue
            from util import PROM_LINE_RE
            if PROM_LINE_RE.match(ln):
                found_numeric = True
                break

        if found_numeric:
            metrics_unexpected.append((pod_name, candidate_ip))
        else:
            metrics_empties.append((pod_name, candidate_ip))

    revert_errors = []
    for rn, orig in originals.items():
        try:
            replace_with_retry(nc_namespace, rn, orig)
        except Exception as e:
            LOG.error("Failed to restore %s: %s", rn, e)
            revert_errors.append((rn, str(e)))

    if revert_errors:
        LOG.error("Failed to revert some NetworkConfigs: %s", revert_errors)

    if metrics_unexpected:
        LOG.error("Unexpectedly succeeded fetching numeric metrics from IPs under nodePort: %s", metrics_unexpected)
        pytest.fail(f"Unexpectedly succeeded fetching numeric metrics from IPs under nodePort: {metrics_unexpected}")

    if not metrics_empties:
        pytest.skip("No candidate IPs found under spec.metricsExporter.nodePort to test; nothing to assert")

    LOG.info(
        "Verified that IPs stored under nodePort did not yield metrics when serviceType=ClusterIP: %s",
        metrics_empties,
    )
