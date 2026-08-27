# Release Notes

## v1.2.1

This release adds OpenShift (OLM) deployment support, configurable base image registries for air-gapped environments, and a modernized RoCE workload image, alongside numerous bug fixes for OpenShift driver management, OLM packaging, and node lifecycle operations.

### Release Highlights

- **Network Operator**
  - Added OLM (Operator Lifecycle Manager) deployment support for OpenShift
  - Added `BaseImageRegistry` and `BaseImageRegistryTLS` CRD fields for configuring custom base image registries in air-gapped or private registry environments, with platform-aware defaults (Docker Hub for Ubuntu, Red Hat registry for OpenShift/RHEL)
  - Added automatic unloading of the in-tree `ionic` kernel driver on OpenShift via `InTreeModulesToRemove`, resolving module conflicts that prevented KMM from loading out-of-tree drivers

- **AMD Host Device CNI**
  - Added fallback path resolution for the `host-device` plugin on OpenShift CoreOS where `/opt/cni` is a broken symlink. The plugin now reads `CNI_PATH` from the environment (set by Multus per the CNI spec) and appends fallback directories (`/opt/cni/bin`, `/var/lib/cni/bin`, `/usr/libexec/cni`), ensuring the delegate binary is found regardless of where it is installed

- **RoCE Workload Image**
  - Modernized RCCL workload image build with a multi-stage Dockerfile that builds UCX, OpenMPI, and RCCL from source, replacing the previous shell-script-driven approach

### Bug Fixes

- Fixed kernel module loading order that caused `NetworkConfig` deletion to hang
- Fixed CoreOS KMM driver builds on OpenShift 4.21.7+ where `dnf` is no longer available in the DTK image
- Fixed Multus config detection to support both `.conf` and `.conflist` formats on K3s with Flannel
- Fixed stale KMM version-module labels persisting on nodes after `NetworkConfig` deletion
- Fixed node label cleanup removing unrelated labels during `NetworkConfig` deletion
- Fixed missing `device-plugin` ServiceAccount for OLM deployments
- Fixed OLM bundle versioning that prevented OpenShift operator upgrades
- Fixed OLM bundle image tagging that prevented successful operator publishing
- Fixed OpenShift Helm chart build failures and cleaned up stale metadata from OLM bundle
- Fixed base image registry configuration not being applied in all build stages
- Fixed `imageRegistryTLS` not being propagated to the KMM module container, leaving TLS settings empty for private registries
- Fixed node labeller reporting `default` profile for all NICs

## v1.2.0

This release adds full AINIC driver stack management with `pds_core` and `tawk_ipc` modules, LIF-level aggregated QP metrics for improved monitoring scalability, and significant improvements to the AMD Host Device CNI plugin including gateway auto-configuration for source-based routing.

### Release Highlights

- **Network Operator**
  - Added `pds_core` and `tawk_ipc` kernel modules to driver management, completing the AINIC driver stack installation alongside the existing `ionic` module
  - Existing `modulesLoadingOrder` is now preserved during operator upgrades, preventing unexpected kernel module reloads on already-running clusters
  - Upgraded base container images to address known CVEs

- **AMD Host Device CNI**
  - Removed file-based logging in favor of standard output logging, eliminating the need for log rotation management. CNI logs can now be accessed via `journalctl` (e.g., `journalctl -f | grep amd-host-device`), aligning with standard CNI logging practices
  - Automatically computes and configures gateway information via static IPAM configuration for /31 IPv4 networks, enabling seamless integration with source-based routing (SBR) CNI plugin for multi-homed pod scenarios
  - Interface naming now follows CNI standard conventions (e.g., `net1`, `net2`) within pods rather than preserving the host interface name, ensuring compatibility when chaining with other CNI plugins and avoiding naming conflicts
  - **Backwards Compatibility**: Automatic fallback mechanism handles pod deletion for workloads created with previous versions. If the DEL operation fails with the standard interface name, the plugin automatically retries with the legacy host interface name, ensuring seamless upgrades without manual intervention

- **Metrics Exporter**
  - Introduced LIF-level aggregated Queue-Pair (QP) metrics, reducing Prometheus metric cardinality and lowering CPU/memory overhead on both the Exporter and Prometheus compared to per-QP metrics
  - Added `ETH_FRAMES_RX_PRIPAUSE`, `ETH_FRAMES_TX_PRIPAUSE`, and `NIC_PORT_STATS_RSFEC_UNCORRECTABLE_WORD` metrics for monitoring priority-level flow control and FEC failures
  - Per-QP metrics (`QP_*`) are now disabled by default; available on-demand via `/metrics?debug=qp`
  - Replaced `crictl` with CRI API for container metadata collection, improving reliability and removing external tool dependency
  - Docker image size reduced by 57% (903MB to 386MB) by stripping debug symbols
  - Standardized ethtool priority field naming to underscore format (`PRI_0` through `PRI_7`). Old format maintained as deprecated aliases; migration recommended before removal in a future release
  - Added `nic_techsupport_dump.sh` script for collecting NIC diagnostics in both standalone Helm and operator-managed deployments

### Bug Fixes

- Fixed driver module loading order issues that prevented the device plugin from starting correctly
- Fixed KMM build failures when builder pods were scheduled to nodes with a different kernel version than the target
- Fixed `libkmod` loading failure by mounting `/lib/modules` into device plugin and metrics exporter pods

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
