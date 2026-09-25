# AMD Host Device CNI Plugin

The AMD Host Device CNI plugin is a specialized Container Network Interface (CNI) plugin that moves Physical Function (PF) or Virtual Function (VF) network interfaces from the host directly into pod network namespaces while preserving their IP addresses. This approach provides pods with direct access to high-performance network interfaces while maintaining network configuration consistency.

## Key Features

### Interface Movement and IP Preservation

- **Direct PF/VF Movement**: Moves entire Physical or Virtual Function interfaces from host to pod namespace
- **IP Address Preservation**: Captures and preserves existing IP addresses (both IPv4 and IPv6 addresses) from the host interface and passes them to the static IPAM configuration
- **Policy route injection (opt-in)**: set both `nexthopNetAddrOffset` and `routeDstPrefixLen` on the NAD to inject a route into the static IPAM result for SBR. See [Policy route injection](../amd-host-device/route-injection.md). Earlier releases implicitly added a gateway for `/31` links; that behaviour is removed. Restore a default route via the peer with `"nexthopNetAddrOffset": 0` and `"routeDstPrefixLen": 0`
- **IP Address and State Persistence**: IP addresses and the interface state are retained on the host interface even after workload deletion

## Configuration

### NetworkAttachmentDefinition

Separate NAD should be created for each resource type: `nic` and `vnic`

#### NAD for `nic`

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

### NAD for `vnic`

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

For detailed information on how this resource is allocated and how the CNI is invoked, please refer to the [integration flow documentation](./integration-flow.md).

### Configuration Reference

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `type` | string | yes | Must be `"amd-host-device"` |
| `cniVersion` | string | yes | CNI spec version (e.g. `"0.3.1"`) |
| `nexthopNetAddrOffset` | integer | no | Offset from subnet network address to derive gateway IP. Ignored on `/31` (XOR peer used). Must be set with `routeDstPrefixLen` |
| `routeDstPrefixLen` | integer (0–32) | no | Prefix length for route destination. Must be less than interface prefix. `0` = default route. Must be set with `nexthopNetAddrOffset` |

### Route Injection (opt-in)

When both `nexthopNetAddrOffset` and `routeDstPrefixLen` are set, the plugin computes a route per IPv4 address on the host interface and injects it into the static IPAM `routes` array. When chained with the [SBR CNI plugin](https://github.com/k8snetworkplumbingwg/sbr-cni), SBR installs the route into a per-source policy routing table. For the full derivation algorithm and edge cases, see [Policy Route Injection](../amd-host-device/route-injection.md).

**Field value combinations:**

| `nexthopNetAddrOffset` | `routeDstPrefixLen` | Effect |
| --- | --- | --- |
| *(absent)* | *(absent)* | No route injected. Interface moves to pod with IPs only. |
| `1` | `19` | Supernet route: `<supernet>/19 via <network_addr+1>`. For leaf/spine fabrics with /25 host links. |
| `1` | `0` | Default route: `0.0.0.0/0 via <network_addr+1>`. All non-local traffic exits via gateway. |
| `0` | `0` | Default route on /31: `0.0.0.0/0 via <XOR peer>`. v1.2.x migration path. On non-/31, gateway = network address (offset 0), generally not useful. |
| `0` | `16` | /31 supernet: `<supernet>/16 via <XOR peer>`. Offset ignored on /31. |

**Key behaviors:**

- Both fields must be set together — setting only one is a no-op (logged as warning)
- `routeDstPrefixLen: 0` means a default route (`0.0.0.0/0`) — routes **all** non-local traffic via the computed gateway
- On `/31` links, `nexthopNetAddrOffset` is always ignored; the XOR peer is used as the gateway regardless of the offset value
- On non-`/31` links, `nexthopNetAddrOffset: 0` produces a gateway equal to the network address (e.g., `.128` for a `/25`) — this is typically not a valid host; use `1` for the first usable address

#### NAD with supernet route (leaf/spine fabric)

For environments with `/25` host interfaces and a `/19` supernet across the fabric (e.g., spine/leaf datacenter topologies), this NAD injects a supernet route so pods can reach peers behind other leaf switches. For a NIC at `10.1.2.130/25`, this installs `10.1.0.0/19 via 10.1.2.129` in the SBR policy table.

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: amd-host-device-sbr-nad
  annotations:
    k8s.v1.cni.cncf.io/resourceName: amd.com/nic
spec:
  config: |-
    {
      "cniVersion": "0.3.1",
      "name": "amd-host-device-sbr-nad",
      "plugins": [
        {
          "type": "amd-host-device",
          "nexthopNetAddrOffset": 1,
          "routeDstPrefixLen": 19
        },
        { "type": "sbr" }
      ]
    }
```

#### NAD with default route (non-/31)

Routes all secondary-network traffic via the first host in the subnet. For a NIC at `10.1.2.130/25`, this installs `0.0.0.0/0 via 10.1.2.129`.

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: amd-host-device-sbr-default-nad
  annotations:
    k8s.v1.cni.cncf.io/resourceName: amd.com/nic
spec:
  config: |-
    {
      "cniVersion": "0.3.1",
      "name": "amd-host-device-sbr-default-nad",
      "plugins": [
        {
          "type": "amd-host-device",
          "nexthopNetAddrOffset": 1,
          "routeDstPrefixLen": 0
        },
        { "type": "sbr" }
      ]
    }
```

#### NAD with default route (/31 — migration from v1.2.x)

Replaces the implicit `/31` gateway that was automatically injected in v1.2.x. For a NIC at `192.168.4.8/31`, this installs `0.0.0.0/0 via 192.168.4.9` (XOR peer). The offset value is ignored on `/31`.

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: amd-host-device-sbr-p2p-nad
  annotations:
    k8s.v1.cni.cncf.io/resourceName: amd.com/nic
spec:
  config: |-
    {
      "cniVersion": "0.3.1",
      "name": "amd-host-device-sbr-p2p-nad",
      "plugins": [
        {
          "type": "amd-host-device",
          "nexthopNetAddrOffset": 0,
          "routeDstPrefixLen": 0
        },
        { "type": "sbr" }
      ]
    }
```

The basic NAD examples shown earlier (without `nexthopNetAddrOffset` and `routeDstPrefixLen`) continue to work unchanged — the interface moves to the pod with its IPs and no routes are injected.

### Verifying Route Injection

When route injection is configured with SBR, verify the injected route appears in the SBR policy table inside the pod. Using the supernet NAD example above (NIC at `10.1.2.130/25`, `nexthopNetAddrOffset: 1`, `routeDstPrefixLen: 19`):

```bash
root@workload-app:/tmp# ip route show table 100
10.1.0.0/19 via 10.1.2.129 dev net1
10.1.2.128/25 dev net1 proto kernel scope link src 10.1.2.130
```

```bash
root@workload-app:/tmp# ip rule show
0:      from all lookup local
32765:  from 10.1.2.130 lookup 100
32766:  from all lookup main
32767:  from all lookup default
```

The first line in table 100 (`10.1.0.0/19 via 10.1.2.129`) is the injected supernet route — the gateway `10.1.2.129` is the network address (`10.1.2.128`) plus the offset (`1`), and the destination `10.1.0.0/19` is the interface IP masked to `/19`. The second line is the connected `/25` subnet route added by static IPAM. The ip rule ensures traffic sourced from the secondary IP (`10.1.2.130`) uses this table.

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
