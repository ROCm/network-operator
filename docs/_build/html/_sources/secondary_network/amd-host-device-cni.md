# AMD Host Device CNI Plugin

The AMD Host Device CNI plugin is a specialized Container Network Interface (CNI) plugin that moves Physical Function (PF) or Virtual Function (VF) network interfaces from the host directly into pod network namespaces while preserving their IP addresses and interface names. This approach provides pods with direct access to high-performance network interfaces while maintaining network configuration consistency.

## Key Features

### Interface Movement and IP Preservation
- **Direct PF/VF Movement**: Moves entire Physical or Virtual Function interfaces from host to pod namespace
- **IP Address Preservation**: Captures and preserves existing IP addresses (both IPv4 and IPv6) from the host interface
- **Interface Name Retention**: Maintains the original host interface name within the pod
- **IP Address and State Persistence**: IP addresses and the interface state are retained on the host interface even after workload deletion

## Configuration

### NetworkAttachmentDefinition

Separate NAD should be created for each resource type: `nic` and `vnic`

#### NAD for `nic`:

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: amd-host-device-nad-nic
  annotations:
    k8s.v1.cni.cncf.io/resourceName: amd.com/nic
spec:
  config: '{
    "name": "amd-host-device-nad-nic",
    "cniVersion": "0.3.1",
    "type": "amd-host-device"
  }'
```

### NAD for `vnic`:

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: amd-host-device-nad-vnic
  annotations:
    k8s.v1.cni.cncf.io/resourceName: amd.com/vnic
spec:
  config: '{
    "name": "amd-host-device-nad-vnic",
    "cniVersion": "0.3.1",
    "type": "amd-host-device"
  }'
```

For detailed information on how this resource is allocated and how the CNI is invoked, please refer to the documentation [here](./integration-flow.md).

## Verification

This section demonstrates how to verify that a RoCE (RDMA over Converged Ethernet) device is correctly allocated to a pod and moved from the host namespace into the pod namespace.

### On the host (Before Allocation)

Check the RoCE device using `rdma` and `ibv_devices`:
```bash
root@genoa4:~# rdma link show rocep68s0/1
link rocep68s0/1 state ACTIVE physical_state LINK_UP netdev enp68s0
root@genoa4:~# ibv_devices | grep rocep68s0
    rocep68s0           069081fffe2c4f90
root@genoa4:~#
```

Check the associated Ethernet interface:
```bash
root@genoa4:~# ifconfig enp68s0
enp68s0: flags=4163<UP,BROADCAST,RUNNING,MULTICAST>  mtu 1500
        inet 55.1.1.56  netmask 255.255.255.0  broadcast 55.1.1.255
        inet6 fe80::690:81ff:fe2c:4f90  prefixlen 64  scopeid 0x20<link>
        ether 04:90:81:2c:4f:90  txqueuelen 1000  (Ethernet)
        RX packets 630705  bytes 70505656 (70.5 MB)
        RX errors 0  dropped 0  overruns 0  frame 0
        TX packets 0  bytes 0 (0.0 B)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0
```

### On the Workload Pod (After Allocation)

Once the RoCE device is allocated to a pod, the device and interface are moved out of the host namespace and become visible inside the pod.

Check inside the pod:

```bash
root@workload-app-nic-679fb76687-wbhlg:/tmp# rdma link show rocep68s0/1
link rocep68s0/1 state ACTIVE physical_state LINK_UP netdev enp68s0
root@workload-app-nic-df886b98c-v5glk:/tmp# ibv_devices
    device                 node GUID
    ------              ----------------
    rocep68s0           069081fffe2c4f90
root@workload-app-nic-df886b98c-v5glk:/tmp#
```

```bash
root@workload-app-nic-679fb76687-wbhlg:/tmp# ifconfig enp68s0
enp68s0: flags=4163<UP,BROADCAST,RUNNING,MULTICAST>  mtu 1500
        inet 55.1.1.56  netmask 255.255.255.0  broadcast 55.1.1.255
        inet6 fe80::690:81ff:fe2c:4f90  prefixlen 64  scopeid 0x20<link>
        ether 04:90:81:2c:4f:90  txqueuelen 1000  (Ethernet)
        RX packets 631105  bytes 70548756 (70.5 MB)
        RX errors 0  dropped 0  overruns 0  frame 0
        TX packets 0  bytes 0 (0.0 B)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0
```

### On the Host (After Allocation)
After allocation, the ethernet interface is no longer present in the host namespace:

```bash
root@genoa4:~# ifconfig enp68s0
enp68s0: error fetching interface information: Device not found
root@genoa4:~#
```