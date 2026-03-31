# Release Notes

## v1.2.0

### Release Highlights

- **AMD Host Device CNI**
  - Removed file-based logging in favor of standard output logging, eliminating the need for log rotation management. CNI logs can now be accessed via `journalctl` (e.g., `journalctl -f | grep amd-host-device`), aligning with standard CNI logging practices
  - Automatically compute and configure gateway information via static IPAM configuration for /31 IPv4 networks, enabling seamless integration with source-based routing (SBR) CNI plugin for multi-homed pod scenarios
  - Interface naming now follows CNI standard conventions (e.g., `net1`, `net2`) within pods rather than preserving the host interface name, ensuring compatibility when chaining with other CNI plugins and avoiding naming conflicts
  - **Backwards Compatibility**: Automatic fallback mechanism handles pod deletion for workloads created with previous versions. If the DEL operation fails with the standard interface name, the plugin automatically retries with the legacy host interface name, ensuring seamless upgrades without manual intervention

## v1.1.0

This release introduces major enhancements, including a Cluster Validation Framework and Network Operator images redesigned for deployment independent of the host OS version.

### Release Highlights

- **Network Operator**
  - Introduced support for the Cluster Validation Framework, enabling validation of newly added worker nodes in the Kubernetes cluster before scheduling distributed training or inference workloads
  - Added support for Fluent sidecar-based logging, providing centralized logging of cluster validation runs.

- **Device Plugin, Metrics Exporter and Node Labeller**
  - The NICCTL tool is now bundled within the Device Plugin, Metrics Exporter and Node Labeller images, allowing these Operator components to run independently of host OS versions

- **RoCE Workload Image**
  - Ubuntu-based workload image with supported AINIC firmware 1.117.5-a-56 has been uploaded to ROCm Docker Hub for running RCCL and InfiniBand tests

## v1.0.1

This release introduces support for user-defined tolerations in KMM modules and includes significant latency improvements for RDMA statistics in the Device Metrics Exporter.

### Release Highlights

- **Network Operator**
  - Added support for user-defined tolerations for the KMM module. Users can now inject custom tolerations into the KMM Module via the NetworkConfig CR.

- **Device Metrics Exporter**
  - Improved RDMA statistics collection, reducing the previously observed latency by several folds compared to the earlier release `v1.0.0`

## v1.0.0

This release is the first major release of AMD Network Operator. The AMD Network Operator simplifies the use of AMD AINICs in Kubernetes environments. It manages all networking components required to enable RDMA workloads within a Kubernetes cluster.

### Release Highlights

- Manage AMD AI NIC drivers with desired versions on Kubernetes cluster nodes
- Customized scheduling and efficient resource allocation for containers
- Metrics and statistics monitoring solution for AMD AI NIC workloads

### Hardware Support

#### New Hardware Support

- **AMD Pensando™ Pollara AI NIC**

### Platform Support

#### New Platform Support

- **Kubernetes 1.29+**
  - Supported features:
    - Driver management
    - Workload scheduling
    - Metrics monitoring
  - Requirements: Kubernetes version 1.29+

### Breaking Changes

Not Applicable as this is the initial release.
</br>
