# Troubleshooting

This guide provides steps to diagnose and resolve common issues with the AMD Network Operator.

## Checking Operator Status

To check the status of the AMD Network Operator:

```bash
kubectl get pods -n kube-amd-network
```

## Collecting Logs

To collect logs from the AMD Network Operator:

```bash
kubectl logs -n kube-amd-network <pod-name>
```

## Using Techsupport-dump Tool

The techsupport-dump tool collects system state and logs for debugging purposes. It can be run from any node in the cluster, including control plane nodes.

```bash
./tools/techsupport_dump.sh [-w] [-o yaml/json] [-k kubeconfig] <node-name/all>
```

Options:

- `-w`: wide option
- `-o yaml/json`: output format (default: json)
- `-k kubeconfig`: path to kubeconfig (default: ~/.kube/config)

### TechSupport Collects

1. **Kubernetes resources** from the `network-operator`, `nfd`, and `kmm` namespaces, including:
   - Pods
   - DaemonSets
   - Deployments
   - ConfigMaps
   - `NetworkConfig` resources

2. **Pod logs** from components such as:

   - Node Feature Discovery (NFD)
   - Kernel Module Management (KMM)
   - Network Operator (Data Plane, Metrics Exporter, CNI plugins)

3. **System-level diagnostics**:

   - `lsmod` output (loaded kernel modules)
   - `dmesg` output (kernel ring buffer)
