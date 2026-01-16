# Overview
This repository contains a pytest-based integration test suite that validates the behavior of a Kubernetes NetworkConfig custom resource and a metrics exporter that exposes Prometheus‑formatted metrics on nodePorts.

## Test Files
- **test_network_operator_metrics_exporter.py** - Core metrics exporter functionality tests
- **test_network_operator_rbac_metrics_exporter.py** - RBAC-enabled metrics exporter tests using mTLS
- **util.py** - Shared utility functions for Kubernetes operations
- **set_env.py** - Environment setup script for copying kubeconfig
- **env.json** - Environment configuration file with cluster details


## Prerequisites 
1. pytest
2. kubernetes Python client
3. Access to a Kubernetes cluster with:
4. The NetworkConfig CRD (kind: NetworkConfig) installed in namespace kube-amd-network
5. Workload pods in the default namespace with names that allow selection for vf-workload vs others
6. The nodes and pods configured so test traffic and curl succeed (e.g., container shell present, curl installed, IB tools if IB traffic is used)
7. IMPORTANT HARD REQUIREMENT: If a workload is running on a VM node, the corresponding workload pod name and the matching NetworkConfig name MUST start with the prefix vf-. VM-hosted workloads require vf-‑prefixed NetworkConfig objects and vf-‑prefixed workload pod names so the tests can match workloads to configs correctly.
8. KUBECONFIG environment variable set (or default kubeconfig available)

## Environment Variables

### util.py
- **KUBECONFIG** — Path to kubeconfig file for cluster authentication (optional, uses default if not set)
- **TEST_MAX_WORKERS** — Maximum concurrency for ThreadPoolExecutor (default: 6)
- **LOCAL_CERT_DIR** — Directory containing mTLS certificates (client.crt, client.key, ca.crt) for RBAC metrics tests (default: ~/certs)

### test_network_operator_rbac_metrics_exporter.py
- **NETWORKCONFIG_NAME** — Name of the NetworkConfig resource to use for RBAC tests (default: "test-networkconfig")

### Environment Configuration File
Create an env.json file in the root directory with your environment details:

```json
{
  "environments": {
    "master-node": {
      "ip": "10.30.29.208",
      "username": "vm",
      "password": "vm"
    },
    "dev": {
      "ip": "10.30.29.209",
      "username": "vm",
      "password": "vm"
    },
    "staging": {
      "ip": "10.30.29.210",
      "username": "vm",
      "password": "vm"
    }
  }
}

## High-level test list (pytest functions)

### test_network_operator_metrics_exporter.py
1. **test_all_pods_running** — enumerate pods and log non-running pods
2. **test_ib_traffic_and_pull_metrics** — run IB traffic in workload pods and validate metrics from nodePorts
3. **test_update_nodeport_and_verify_metrics_pull** — patch nodePorts, validate metrics, revert
4. **test_disable_metrics_exporter_and_verify_no_metrics** — disable exporters and assert no numeric metrics returned
5. **test_out_of_range_nodeport** — ensure out-of-range nodePort is rejected
6. **test_pull_metrics_using_source_port** — curl metrics using --local-port from pod to nodePort
7. **test_custom_source_port** — test custom source port configuration

### test_network_operator_rbac_metrics_exporter.py
1. **test_rbac_node_port** — test RBAC-enabled metrics exporter with mTLS using curl_metrics_from_local

## Key helper functions (util.py)

### Kubernetes Configuration
- **load_kube_config()** — Load kubeconfig for cluster authentication

### NetworkConfig CRD Operations
- **discover_networkconfig_crd()** — Discover NetworkConfig CRD details (group, version, plural)
- **list_networkconfigs_custom(namespace)** — List all NetworkConfig custom resources in namespace
- **get_networkconfig_custom(namespace, name)** — Get specific NetworkConfig CR by name
- **replace_networkconfig_custom(namespace, name, body)** — Replace NetworkConfig CR with new body
- **patch_networkconfig_custom(namespace, name, patch_body)** — Patch NetworkConfig CR with partial updates
- **replace_with_retry(namespace, name, body, max_attempts=5, backoff=0.5)** — Replace resource with retries on 409 conflicts

### Pod and Node Operations
- **list_pods(v1, namespace=None)** — List all pods in namespace
- **list_workloads(v1, namespace="default")** — List workload pods (Running, Pending, or Unknown)
- **get_node_ip(v1, node_name, max_retries=3, retry_delay=1.0)** — Get node InternalIP with retry logic
- **exec_in_pod_sync(v1, pod_name, namespace, cmd, timeout=120)** — Execute command synchronously in pod

### InfiniBand and Metrics
- **run_ib_traffic(v1, pod_name, ns)** — Run InfiniBand traffic test inside pod using COMPOSITE_IB_CMD
- **pull_metrics(v1, pod_name, ns, port, node_ip)** — Curl metrics from http://nodeIP:port/metrics from a pod
- **node_metrics_have_numeric(v1, pod_name, ns, node_ip, port)** — Check if metrics endpoint returns numeric Prometheus lines
- **wait_for_metrics_ready(v1, pod_name, ns, node_ip, port, timeout=30, interval=1.0)** — Wait for metrics to be available with polling
- **get_metrics_nodeport_from_networkconfig(v1_custom, namespace, name)** — Extract metrics nodePort from NetworkConfig spec

### mTLS and Local Metrics
- **curl_metrics_from_local(node_ip, node_port, cert_dir=LOCAL_CERT_DIR, timeout=10)** — Fetch metrics from local machine using mTLS certificates (client.crt, client.key, ca.crt)
- **_run_local(cmd, timeout=10)** — Run shell command locally and return (returncode, stdout, stderr)

### Prometheus Integration
- **detect_prometheus_base_url_from_k8s(namespace="monitoring")** — Auto-detect Prometheus service URL from Kubernetes cluster

### Constants
- **PROM_LINE_RE** — Regex pattern to validate Prometheus metric lines
- **TEST_TIMEOUT** — 180 seconds per test timeout
- **COMPOSITE_IB_CMD** — Shell command for running IB traffic tests
- **MAX_WORKERS** — Thread pool size (default 6, configurable via TEST_MAX_WORKERS env var)
- **LOCAL_CERT_DIR** — Directory for mTLS certificates (default: ~/certs, configurable via LOCAL_CERT_DIR env var)



## Test cases (inputs / expected outputs)

### test_network_operator_metrics_exporter.py

1. **test_all_pods_running**
Input: Kubernetes cluster with multiple pods
Action: enumerate pods; log non-running pods
Expected outcome: test runs and completes (it logs non-running pods but does not fail)

2. **test_ib_traffic_and_pull_metrics**
Input:
NetworkConfig CRs in kube-amd-network with spec.metricsExporter.nodePort
At least one Running pod in default
NOTE: VM-hosted workloads require vf- prefix on both workload pod name and corresponding NetworkConfig name
Action: run IB traffic in each running workload pod; curl http://nodeIP:nodePort/metrics; validate numeric Prometheus lines
Expected outcome: metrics validated for each pod; failure if any pod returns no numeric metrics, skip if no running workloads or cannot list NetworkConfig

3. **test_update_nodeport_and_verify_metrics_pull**
Input: NetworkConfig CRs exist and are editable; Running workload pods
Action: store originals; patch nodePorts to 32520 (vf-*) / 32521 (others); validate metrics; revert originals
Expected outcome: metrics reachable at updated ports; originals restored; test fails on patch failures or missing metrics

4. **test_disable_metrics_exporter_and_verify_no_metrics**
Input: NetworkConfig objects with nodePort set; Running workload pods
Action: patch spec.metricsExporter.enable=false; run IB traffic; curl original nodePorts and verify no numeric metrics; restore originals
Expected outcome: no numeric metrics returned; test fails if metrics are still present

5. **test_out_of_range_nodeport**
Input: NetworkConfig CRs
Action: attempt to set nodePort=32800 (outside default NodePort range 30000–32767); expect API validation rejection
Expected outcome: all patches rejected; originals restored; test fails if any patch accepted

6. **test_pull_metrics_using_source_port**
Input: NetworkConfig objects that include both spec.metricsExporter.nodePort and spec.metricsExporter.port (source port)
Action: remove nodePort from CR (patch to None), run IB traffic, from each workload pod run curl using --local-port <source-port> to http://nodeIP:nodePort/metrics, validate numeric Prometheus lines, restore originals
Expected outcome: metrics available when using the configured source port; test fails if missing or restore fails

7. **test_custom_source_port**
Input: NetworkConfig objects with configurable source port
Action: test custom source port configuration for metrics exporter
Expected outcome: metrics accessible using custom configured source port

### test_network_operator_rbac_metrics_exporter.py

1. **test_rbac_node_port**
Input: NetworkConfig CRs with RBAC enabled; mTLS certificates (client.crt, client.key, ca.crt) in LOCAL_CERT_DIR
Action: enable metrics exporter with RBAC; run curl_metrics_from_local to fetch metrics using mTLS from local machine
Expected outcome: metrics successfully retrieved with mTLS authentication; test fails if certificates missing or metrics unreachable

## Running the tests 
### Use set_env.py to automatically copy kubeconfig
python set_env.py ripen

### Run all tests in a specific file
pytest -q test_network_operator_metrics_exporter.py
pytest -q test_network_operator_rbac_metrics_exporter.py

### Run a specific test
pytest -q test_network_operator_metrics_exporter.py::test_ib_traffic_and_pull_metrics
pytest -q test_network_operator_rbac_metrics_exporter.py::test_rbac_node_port

### Run all tests in the directory
pytest -q .
