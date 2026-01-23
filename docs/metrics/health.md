# Health Checks

Health monitoring is performed by the metrics exporter, which exposes health data through a gRPC socket for client consumption.
The Kubernetes Device Plugin utilizes this health socket to determine AMD AINIC resource availability for Kubernetes pod scheduling.

Both the device plugin and metrics exporter components must be enabled through the Network Operator for this feature to function.

**Note:** Health monitoring does not currently work in virtualized environments. The exporter pod must run on a non-VM Kubernetes node to obtain interface health information.

## Health Check Workflow

To enable the health check workflow, set the `InterfaceAdminDownAsUnhealthy` field under `HealthCheckConfig` within the `NICConfig` section of `config.json` (in the metrics exporter ConfigMap). If this field is not set, or is set to `false`, all NIC/vNIC resources are reported as healthy by default.

For more details on configuring the metrics exporter, refer to the [Advanced configuration](./exporter.md#advanced-configuration) section.

Once health monitoring is enabled, when the device plugin requests the health status, the metrics exporter reports a NIC/vNIC resource as unhealthy if the associated interface is down.

## Resource Health Status

Resource health status directly affects its availability on Kubernetes nodes. You can verify the current status by running `kubectl describe node <node_name>`.

The node's resource information reflects the current health status of the attached NIC and vNIC resources:

- **Capacity**: Displays the total number of NIC and vNIC resources physically present on the node.
- **Allocatable**: Displays the total number of allocatable NIC and vNIC resources on the node. An unhealthy resource is not allocatable.

### Healthy Resources

When the associated interface for a NIC/vNIC resource is functioning properly, the resource is considered healthy and is available for pod scheduling. This means Kubernetes can assign workloads to the node having these resources without restriction.

For example, on a node with a single healthy NIC and vNIC resource:

```yaml
Capacity:
  amd.com/nic: 1
  amd.com/vnic: 1
Allocatable:
  amd.com/nic: 1
  amd.com/vnic: 1
```

### Unhealthy Resources

If a NIC/vNIC resource is reported as unhealthy, it is no longer available for scheduling new pods. However, any existing pods already using the resource will continue to run without interruption. This ensures that running workloads are not disrupted, but new workloads requiring a healthy resource will not be scheduled on the affected node until the resource returns to a healthy state.

**Key behaviors:**

- New pods requesting AMD NIC/vNIC resource cannot be scheduled on nodes where all such resources are unhealthy.
- Pods that require NIC/vNIC resource will remain in a pending state until a healthy resource becomes available.
- The node's allocatable resource count is reduced by the number of unhealthy resources, while the total capacity remains unchanged.

For example, if a node has a single NIC and vNIC resource and both of them become unhealthy, the allocatable count will decrease to 0:

```yaml
Capacity:
  amd.com/nic: 1
  amd.com/vnic: 1
Allocatable:
  amd.com/nic: 0
  amd.com/vnic: 0
```

The node labels also provide this information. Each resource's health state is tracked using the following label:

```yaml
metricsexporter.amd.com.nic.<IFACE_PCI_ADDR>.state=unhealthy
```
