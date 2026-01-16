#!/usr/bin/env python3

import os
import re
import time
import logging
import yaml
from typing import List, Dict, Any, Optional, Tuple, Any as AnyType
from concurrent.futures import ThreadPoolExecutor
from kubernetes import config
from kubernetes.client.rest import ApiException
from kubernetes.stream import stream
from kubernetes import client as k8s_client
import subprocess
from pathlib import Path

LOG = logging.getLogger("test_network_operator_metrics_exporter")
logging.basicConfig(level=logging.INFO)

# ---------- Constants ----------
PROM_LINE_RE = re.compile(
    r"^\s*([a-zA-Z_:][a-zA-Z0-9_:]*)\s*(\{.*\})?\s+(-?\d+(\.\d+)?([eE][-+]?\d+)?)\s*$"
)
TEST_TIMEOUT = 180  # seconds per test
COMPOSITE_IB_CMD = (
    "timeout 60 ib_write_bw -d ionic_0 -i 1 -n 1000 -F -a -x 1 -q 1 -b & "
    "sleep 3 && "
    "ib_write_bw -d ionic_0 -i 1 -n 1000 -F -a -x 1 -q 1 -b localhost ; "
    "pkill -9 ib_write_bw"
)
MAX_WORKERS = int(os.environ.get("TEST_MAX_WORKERS", "6"))

_crd_cache: Optional[Tuple[str, str, str]] = None

LOCAL_CERT_DIR = os.environ.get("LOCAL_CERT_DIR", str(Path.home() / "certs"))

def load_kube_config() -> None:
    cfg_path = os.environ.get("KUBECONFIG")
    config.load_kube_config(config_file=cfg_path)


def discover_networkconfig_crd() -> Tuple[str, str, str]:
    """
    Returns (group, version, plural) for NetworkConfig CRD. Cached.
    """
    global _crd_cache
    if _crd_cache:
        return _crd_cache
    ext_api = k8s_client.ApiextensionsV1Api()
    crds = ext_api.list_custom_resource_definition().items
    for crd in crds:
        try:
            if crd.spec.names.kind == "NetworkConfig":
                versions = crd.spec.versions or []
                chosen = None
                for v in versions:
                    if getattr(v, "served", False):
                        chosen = v.name
                        if getattr(v, "storage", False):
                            break
                if not chosen and versions:
                    chosen = versions[0].name
                _crd_cache = (crd.spec.group, chosen, crd.spec.names.plural)
                return _crd_cache
        except Exception:
            continue
    raise RuntimeError("NetworkConfig CRD (kind=NetworkConfig) not found")


def list_networkconfigs_custom(namespace: str) -> List[Dict[str, Any]]:
    group, version, plural = discover_networkconfig_crd()
    co_api = k8s_client.CustomObjectsApi()
    resp = co_api.list_namespaced_custom_object(group=group, version=version, namespace=namespace, plural=plural)
    return resp.get("items", [])


def get_networkconfig_custom(namespace: str, name: str) -> Dict[str, Any]:
    group, version, plural = discover_networkconfig_crd()
    co_api = k8s_client.CustomObjectsApi()
    return co_api.get_namespaced_custom_object(group=group, version=version, namespace=namespace, plural=plural, name=name)


def replace_networkconfig_custom(namespace: str, name: str, body: Dict[str, Any]) -> None:
    group, version, plural = discover_networkconfig_crd()
    co_api = k8s_client.CustomObjectsApi()
    co_api.replace_namespaced_custom_object(group=group, version=version, namespace=namespace, plural=plural, name=name, body=body)


def patch_networkconfig_custom(namespace: str, name: str, patch_body: Dict[str, Any]) -> None:
    group, version, plural = discover_networkconfig_crd()
    co_api = k8s_client.CustomObjectsApi()
    co_api.patch_namespaced_custom_object(group=group, version=version, namespace=namespace, plural=plural, name=name, body=patch_body)


def replace_with_retry(namespace: str, name: str, body: Dict[str, Any], max_attempts: int = 5, backoff: float = 0.5):
    last_exc = None
    for attempt in range(1, max_attempts + 1):
        try:
            replace_networkconfig_custom(namespace, name, body)
            return
        except ApiException as e:
            last_exc = e
            if getattr(e, "status", None) == 409:
                LOG.warning("409 conflict replacing %s/%s attempt %d/%d; fetching latest resourceVersion", namespace, name, attempt, max_attempts)
                try:
                    latest = get_networkconfig_custom(namespace, name)
                    body.setdefault("metadata", {})["resourceVersion"] = latest.get("metadata", {}).get("resourceVersion")
                except Exception as ge:
                    LOG.error("Failed to fetch latest %s/%s: %s", namespace, name, ge)
                time.sleep(backoff * attempt)
                continue
            raise
    raise last_exc


def list_pods(v1: k8s_client.CoreV1Api, namespace: Optional[str] = None) -> List[AnyType]:
    try:
        if namespace:
            return v1.list_namespaced_pod(namespace=namespace, watch=False).items
        return v1.list_pod_for_all_namespaces(watch=False).items
    except ApiException as e:
        LOG.error("Error listing pods: %s", e)
        raise


def list_workloads(v1: k8s_client.CoreV1Api, namespace: str = "default") -> List[AnyType]:
    try:
        pods = v1.list_namespaced_pod(namespace=namespace, watch=False).items
        running = [p for p in pods if getattr(p.status, "phase", None) == "Running"]
        LOG.info("Found %d running workload pods in namespace %s", len(running), namespace)
        return running
    except Exception as e:
        LOG.error("Failed to list workloads in %s: %s", namespace, e)
        raise


def get_node_ip(v1: k8s_client.CoreV1Api, node_name: str, max_retries: int = 3, retry_delay: float = 1.0) -> Optional[str]:
    if not node_name:
        LOG.error("node_name is None or empty")
        return None
    
    for attempt in range(1, max_retries + 1):
        try:
            node = v1.read_node(name=node_name)
            for addr in node.status.addresses:
                if addr.type == "InternalIP":
                    return addr.address
            LOG.warning("Attempt %d/%d: No InternalIP found for node %s", attempt, max_retries, node_name)
        except ApiException as e:
            LOG.warning("Attempt %d/%d: Failed to read node %s: %s", attempt, max_retries, node_name, e)
        
        if attempt < max_retries:
            time.sleep(retry_delay)
    
    LOG.error("Failed to get node IP for %s after %d attempts", node_name, max_retries)
    return None


def exec_in_pod_sync(v1: k8s_client.CoreV1Api, pod_name: str, namespace: str, cmd: str, timeout: int = 120) -> str:
    return stream(
        v1.connect_get_namespaced_pod_exec,
        pod_name,
        namespace,
        command=["/bin/sh", "-c", cmd],
        stderr=True,
        stdin=False,
        stdout=True,
        tty=False,
        _request_timeout=timeout,
    ) or ""


def run_ib_traffic(v1: k8s_client.CoreV1Api, pod_name: str, ns: str) -> Dict[str, str]:
    LOG.info("Running IB in pod %s/%s", ns, pod_name)
    try:
        out = exec_in_pod_sync(v1, pod_name, ns, COMPOSITE_IB_CMD, timeout=60)
        return {pod_name: out or ""}
    except Exception as e:
        LOG.error("IB exec failed in %s/%s: %s", ns, pod_name, e)
        return {pod_name: f"ERROR: {e}"}


def pull_metrics(v1: k8s_client.CoreV1Api, pod_name: str, ns: str, port: int, node_ip: str) -> Optional[str]:
    curl_cmd = f"curl -sS --connect-timeout 3 http://{node_ip}:{port}/metrics || true"
    try:
        return exec_in_pod_sync(v1, pod_name, ns, curl_cmd, timeout=5)
    except Exception as e:
        LOG.error("pull_metrics exec failed for %s: %s", pod_name, e)
        return None


def node_metrics_have_numeric(v1, pod_name: str, ns: str, node_ip: str, port: int) -> bool:
    txt = pull_metrics(v1, pod_name, ns, port, node_ip)
    if not txt:
        return False
    for ln in txt.splitlines():
        ln = ln.strip()
        if not ln or ln.startswith("#"):
            continue
        if PROM_LINE_RE.match(ln):
            return True
    return False


def wait_for_metrics_ready(v1, pod_name: str, ns: str, node_ip: str, port: int, timeout: int = 30, interval: float = 1.0) -> bool:
    start = time.time()
    while time.time() - start < timeout:
        if node_metrics_have_numeric(v1, pod_name, ns, node_ip, port):
            return True
        time.sleep(interval)
    return False

def get_metrics_nodeport_from_networkconfig(v1_custom: k8s_client.CustomObjectsApi,
                                            namespace: str,
                                            name: str) -> Optional[int]:
    """
    Read the NetworkConfig custom object and return the value of spec.metricsExporter.nodePort as int.
    Returns None if the field is missing or cannot be parsed as int.
    v1_custom should be an instance of kubernetes.client.CustomObjectsApi().
    """
    try:
        nc = v1_custom.get_namespaced_custom_object(
            group=discover_networkconfig_crd()[0],
            version=discover_networkconfig_crd()[1],
            namespace=namespace,
            plural=discover_networkconfig_crd()[2],
            name=name,
        )
    except ApiException as e:
        LOG.error("Failed to get NetworkConfig %s/%s: %s", namespace, name, e)
        return None
    except Exception as e:
        LOG.error("Unexpected error fetching NetworkConfig %s/%s: %s", namespace, name, e)
        return None

    # Walk the dict safely to the metricsExporter.nodePort
    try:
        me = nc.get("spec", {}).get("metricsExporter", {}) or {}
        node_port_val = me.get("nodePort")
        if node_port_val is None:
            LOG.warning("NetworkConfig %s/%s: spec.metricsExporter.nodePort is missing", namespace, name)
            return None
        # If it's already an int, return it
        if isinstance(node_port_val, int):
            return node_port_val
        # If it's a string that looks like an integer, parse it
        if isinstance(node_port_val, str):
            node_port_val = node_port_val.strip()
            if node_port_val.isdigit():
                return int(node_port_val)
            # allow strings like "32500" with whitespace; reject IP-like values
            try:
                return int(node_port_val)
            except Exception:
                LOG.error("NetworkConfig %s/%s: spec.metricsExporter.nodePort value (%r) is not an integer", namespace, name, node_port_val)
                return None
        # Unexpected type
        LOG.error("NetworkConfig %s/%s: spec.metricsExporter.nodePort has unexpected type %s", namespace, name, type(node_port_val))
        return None
    except Exception as e:
        LOG.error("Error extracting nodePort from NetworkConfig %s/%s: %s", namespace, name, e)
        return None

def _run_local(cmd: list, timeout: int = 10) -> Tuple[int, str, str]:
 
    try:
        proc = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                              universal_newlines=True, timeout=timeout)
        return proc.returncode, (proc.stdout or ""), (proc.stderr or "")
    except subprocess.TimeoutExpired:
        LOG.error("Local command timed out: %s", " ".join(cmd))
        return -1, "", "timeout"
    except Exception as e:
        LOG.error("Local command failed: %s", e)
        return -1, "", str(e)


def curl_metrics_from_local(node_ip: str,
                            port: int,
                            service_name: str,
                            client_crt: Optional[str] = None,
                            client_key: Optional[str] = None,
                            ca_crt: Optional[str] = None,
                            extra_headers: Optional[dict] = None,
                            timeout: int = 10) -> Optional[str]:
   
    client_crt = client_crt or os.path.join(LOCAL_CERT_DIR, "client.crt")
    client_key = client_key or os.path.join(LOCAL_CERT_DIR, "client.key")
    ca_crt = ca_crt or os.path.join(LOCAL_CERT_DIR, "ca.crt")

    # Verify cert files exist
    missing = []
    for p in (client_crt, client_key, ca_crt):
        if not os.path.isfile(p):
            missing.append(p)
    if missing:
        LOG.error("Missing local cert files required for mTLS curl: %s", missing)
        return None

    # Build curl command
    cmd = [
        "curl",
        "--cert", client_crt,
        "--key", client_key,
        "--cacert", ca_crt,
        "-sS",
        "-H", "Accept: */*",
        "--resolve", f"{service_name}:{port}:{node_ip}",
        f"https://{node_ip}:{port}/metrics"
    ]

    # Add extra headers if provided
    if extra_headers:
        for k, v in extra_headers.items():
            cmd.extend(["-H", f"{k}: {v}"])

    rc, out, err = _run_local(cmd, timeout=timeout)
    if rc != 0:
        LOG.warning("Local curl returned rc=%s stderr=%s", rc, err)
    # Return stdout (may be empty)
    return out or None


def detect_prometheus_base_url_from_k8s(namespace: str = "monitoring") -> Optional[str]:
    """
    Use the Kubernetes API to find a Service in `namespace` that exposes port 9090.
    Returns a URL string like "http://<cluster-ip>:9090" or None if not found.
    Tries in-cluster config first, then kubeconfig from default location.
    """
    # Load Kubernetes configuration

    v1=k8s_client.CoreV1Api
    try:
        svcs = v1.list_namespaced_service(namespace=namespace)
    except ApiException:
        return None

    for svc in svcs.items:
        # Skip headless services (cluster_ip == "None")
        cluster_ip = svc.spec.cluster_ip
        if not cluster_ip or cluster_ip.lower() == "none":
            continue

        # inspect ports to find one exposing 9090 (or named prometheus)
        if svc.spec.ports:
            for p in svc.spec.ports:
                # p.port is integer (target port). We check if it's 9090 or the port name contains 'prom'
                if p.port == 9090 or (p.name and "prom" in p.name.lower()):
                    return f"http://{cluster_ip}:9090/api/v1/query"

    return None
