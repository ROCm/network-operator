# AMD AINIC Network Operator

The AMD AINIC Network Operator simplifies the deployment and management of AMD AI Network Interface Cards (AINICs) in Kubernetes environments. This operator automates the installation, configuration, and lifecycle management of AINIC drivers and network resources.

## Features

- **Automated Driver Management**: Automatically deploys and manages AINIC drivers across Kubernetes nodes
- **SR-IOV Support**: Configures SR-IOV Virtual Functions for high-performance networking
- **DPDK Integration**: Enables DPDK-based networking for ultra-low latency applications
- **Resource Management**: Manages hugepages, CPU, and memory resources for optimal AINIC performance
- **VLAN Configuration**: Supports multiple VLAN configurations for network segmentation
- **Node Selection**: Flexible node targeting using Kubernetes node selectors
- **Status Monitoring**: Provides detailed status information about AINIC deployments

## Architecture

The Network Operator consists of:

1. **AINIC Custom Resource Definition (CRD)**: Defines the desired state of AINIC configurations
2. **Controller**: Reconciles the desired state by managing DaemonSets and driver deployments
3. **Driver DaemonSet**: Runs on selected nodes to configure and manage AINIC hardware
4. **RBAC Configuration**: Provides necessary permissions for operator functionality

## Prerequisites

- Kubernetes cluster (version 1.20+)
- AMD AINIC hardware installed on target nodes
- Nodes labeled appropriately for AINIC deployment
- Sufficient privileges to create CRDs and RBAC resources

## Installation

### Quick Start

1. **Install the CRDs and operator:**
   ```bash
   kubectl apply -f https://github.com/ROCm/network-operator/releases/latest/download/network-operator.yaml
   ```

### From Source

1. **Clone the repository:**
   ```bash
   git clone https://github.com/ROCm/network-operator.git
   cd network-operator
   ```

2. **Build and deploy:**
   ```bash
   make deploy IMG=network-operator:latest
   ```

## Usage

### Basic AINIC Configuration

Create an AINIC resource to deploy drivers with basic SR-IOV configuration:

```yaml
apiVersion: network.amd.com/v1
kind: AINIC
metadata:
  name: ainic-basic
  namespace: default
spec:
  nodeSelector:
    kubernetes.io/arch: amd64
  driver:
    image: "amd/ainic-driver:latest"
    version: "1.0.0"
    args:
      - "--log-level=info"
      - "--enable-sriov"
    env:
      - name: "AINIC_MODE"
        value: "performance"
  networkConfig:
    networkMode: "SR-IOV"
    vfs: 8
    mtu: 9000
    vlan:
      - id: 100
        priority: 1
  resources:
    memory: "2Gi"
    cpu: "2"
    hugepagesSize: "1Gi"
    hugepagesCount: 4
```

### DPDK Configuration

For DPDK-based high-performance networking:

```yaml
apiVersion: network.amd.com/v1
kind: AINIC
metadata:
  name: ainic-dpdk
  namespace: default
spec:
  nodeSelector:
    feature.node.kubernetes.io/network-ainic: "true"
  driver:
    image: "amd/ainic-driver:latest"
    version: "1.0.0"
    args:
      - "--log-level=debug"
      - "--enable-dpdk"
      - "--dpdk-driver=vfio-pci"
    env:
      - name: "AINIC_MODE"
        value: "dpdk"
      - name: "DPDK_HUGEPAGES"
        value: "4096"
  networkConfig:
    networkMode: "DPDK"
    vfs: 16
    mtu: 9000
  resources:
    memory: "4Gi"
    cpu: "4"
    hugepagesSize: "2Mi"
    hugepagesCount: 2048
```

### Checking Status

Monitor the deployment status:

```bash
# List all AINIC resources
kubectl get ainics

# Get detailed status
kubectl describe ainic ainic-basic

# Check driver pods
kubectl get pods -l app=ainic-basic-ainic-driver
```

## Configuration Reference

### AINIC Specification

| Field | Type | Description |
|-------|------|-------------|
| `nodeSelector` | `map[string]string` | Kubernetes node selector for targeting specific nodes |
| `driver` | `DriverSpec` | Driver configuration specification |
| `networkConfig` | `NetworkConfigSpec` | Network-related configuration |
| `resources` | `ResourcesSpec` | Resource allocation specification |

### Driver Specification

| Field | Type | Description |
|-------|------|-------------|
| `image` | `string` | Container image for AINIC driver |
| `version` | `string` | Version of the driver |
| `args` | `[]string` | Additional arguments for the driver |
| `env` | `[]EnvVar` | Environment variables for the driver |

### Network Configuration

| Field | Type | Description |
|-------|------|-------------|
| `networkMode` | `string` | Network mode: "SR-IOV" or "DPDK" |
| `vfs` | `int32` | Number of Virtual Functions to create |
| `mtu` | `int32` | Maximum Transmission Unit size |
| `vlan` | `[]VLANConfig` | VLAN configuration list |

### Resource Specification

| Field | Type | Description |
|-------|------|-------------|
| `memory` | `string` | Memory allocation (e.g., "2Gi") |
| `cpu` | `string` | CPU allocation (e.g., "2") |
| `hugepagesSize` | `string` | Hugepages size (e.g., "2Mi", "1Gi") |
| `hugepagesCount` | `int32` | Number of hugepages to allocate |

## Monitoring and Troubleshooting

### Status Conditions

The operator provides detailed status information:

- **Phase**: Current deployment phase (Pending, Progressing, Ready, Failed)
- **Conditions**: Detailed condition information with reasons and messages
- **Node Counts**: Number of ready vs. total targeted nodes

### Common Issues

1. **Driver pods not starting**: Check node selector and ensure nodes have AINIC hardware
2. **Insufficient resources**: Verify hugepages and memory availability on target nodes
3. **Permission issues**: Ensure proper RBAC configuration and service account permissions

### Logs

Check operator logs:
```bash
kubectl logs -n network-operator-system deployment/network-operator-controller-manager
```

Check driver logs:
```bash
kubectl logs daemonset/ainic-basic-ainic-driver
```

## Development

### Building from Source

```bash
# Generate manifests and code
make manifests generate

# Build the manager binary
make build

# Build Docker image
make docker-build IMG=network-operator:latest

# Run tests
make test
```

### Running Locally

```bash
# Install CRDs
make install

# Run the controller locally
make run
```

## Contributing

Contributions are welcome! Please read our contributing guidelines and submit pull requests to the main branch.

## License

This project is licensed under the Apache License 2.0. See the [LICENSE](LICENSE) file for details.

## Support

For support and questions:

- File issues on [GitHub Issues](https://github.com/ROCm/network-operator/issues)
- Review documentation and examples in this repository
- Check the AMD ROCm community resources