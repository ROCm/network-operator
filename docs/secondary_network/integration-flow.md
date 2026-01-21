# Kubernetes Integration Flow

This document explains how a pod specification with network attachment annotations triggers the CNI plugin through the Kubernetes networking stack.

## Pod Configuration

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: network-workload
  annotations:
    # Refers to the same NetworkAttachmentDefinition (NAD) twice to attach two interfaces from the same resource (amd.com/nic: 2)
    k8s.v1.cni.cncf.io/networks: amd-host-device-nad-nic, amd-host-device-nad-nic
spec:
  containers:
  - name: app
    image: ubuntu:22.04
    resources:
      requests:
        amd.com/nic: 2  # Requests AMD NIC resource
      limits:
        amd.com/nic: 2
```

## Integration Flow

The process involves four main phases:

### 1. Resource Allocation

```text
Pod Request → Scheduler → Device Plugin → NIC Assignment
```

- **Scheduler**: Finds nodes with available `amd.com/nic` resources
- **Device Plugin**: Allocates specific NIC device (e.g., `0000:01:00.0`)
- **Environment**: Device ID passed to pod via `PCIDEVICE_AMD_COM_NIC_0`

### 2. Network Setup Initiation

```text
Kubelet → Multus CNI (from /etc/cni/net.d) → Primary/Cluster Network + Secondary Networks
```

- Kubelet creates container and network namespace
- **Multus is invoked first** (highest priority config in `/etc/cni/net.d/`)
- Multus configures primary network through its delegate configuration
- Multus then processes secondary network attachments from pod annotations

### 3. Multus Orchestration

```yaml
# NetworkAttachmentDefinition
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: amd-host-device-nad
  annotations:
    k8s.v1.cni.cncf.io/resourceName: amd.com/nic  # Links to device resource
spec:
  config: '{
    "name": "amd-host-device-nad",
    "cniVersion": "0.3.1",
    "type": "amd-host-device"  # Specifies CNI plugin to invoke
  }'
```

**Multus Process**:

- Reads `k8s.v1.cni.cncf.io/networks` annotation from pod
- Fetches corresponding NetworkAttachmentDefinition
- Maps device allocation to CNI configuration using resource annotation
- Invokes the CNI with device-specific parameters

### 4. AMD Host Device CNI Execution

**CNI Configuration**:

```json
{
  "name": "amd-host-device-nad",
  "cniVersion": "0.3.1", 
  "type": "amd-host-device",
  "deviceID": "0000:01:00.0"  # Injected by Multus from device allocation
}
```

**Execution Steps**:

1. **IP Capture**: Extract existing IP addresses from host interface
2. **Interface Movement**: Delegate to `host-device` plugin to move interface to pod with the extracted IP addresses
3. **State Persistence**: Store interface-to-IP mapping for lifecycle management

## Key Integration Points

### Device Plugin ↔ CNI Communication

- Device Plugin allocates PCI device ID
- Multus extracts device ID from environment variables
- Device ID passed to CNI for interface identification

### Resource Annotation Mapping

- `k8s.v1.cni.cncf.io/resourceName` links NetworkAttachmentDefinition to Device Plugin resource
- Enables automatic device allocation to CNI parameter mapping
- Provides resource-aware network attachment

## Result

Upon successful completion:

- Pod has direct access to physical/virtual NIC hardware
