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

## Verifying Driver Modules

On OpenShift with KMM, verify whether the loaded kernel module is the out-of-tree
(KMM-managed) version or the in-box RHEL version by comparing `srcversion`:

```bash
# What's running in memory
cat /sys/module/ionic/srcversion

# In-box version on disk
modinfo -F srcversion /lib/modules/$(uname -r)/kernel/drivers/net/ethernet/pensando/ionic/ionic.ko.xz

# Out-of-tree version (if on disk)
modinfo -F srcversion /opt/lib/modules/$(uname -r)/extra/ionic.ko
```

If the loaded srcversion matches the in-box file, the KMM out-of-tree driver did not
load correctly.

> **Note:** `modinfo -F filename ionic` always shows the on-disk in-box path regardless of
> which version is actually loaded in memory. Do not use it to determine the active driver.
