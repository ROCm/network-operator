# Alternative CNI Plugins

This document provides guidance on using different CNI plugins that have been tested with AMD Network Operator as alternatives to the AMD Host Device CNI plugin. These CNI plugins can be used with `NetworkAttachmentDefinition` to address specific networking use cases.

The AMD Network Operator supports multiple CNI plugins through NetworkAttachmentDefinition configurations, below are the few CNIs that we were tested:

- **SR-IOV CNI**: For high-performance networking using Virtual Functions (VFs)
- **RDMA CNI**: For RDMA workloads requiring network namespace isolation
- **SBR CNI**: For Source-Based Routing to enable policy-based routing for secondary networks

## SR-IOV CNI Plugin

The SR-IOV CNI plugin enables moving SR-IOV Virtual Functions (VFs) directly to pod network namespaces, providing high-performance networking with minimal overhead.

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: sriov-nad
  namespace: default
  annotations:
    k8s.v1.cni.cncf.io/resourceName: amd.com/vnic
spec:
  config: |-
    {
      "cniVersion": "0.3.1",
      "name": "sriov-nad",
      "type": "sriov",
      "ipam": {
        "type": "whereabouts",
        "range": "51.3.1.0/24"
      }
    }
```

## RDMA CNI Plugin

The [RDMA CNI plugin](https://github.com/k8snetworkplumbingwg/rdma-cni) provides network namespace isolation for RDMA workloads in containerized environments. It must be chained with other CNI plugins that handle the actual interface movement.

### SR-IOV + RDMA + Tuning Example

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: sriov-rdma-nad
  namespace: default
  annotations:
    k8s.v1.cni.cncf.io/resourceName: amd.com/vnic
spec:
  config: |-
    {
      "cniVersion": "1.0.0",
      "name": "sriov-rdma-nad",
      "plugins": [
        {
          "type": "sriov",
          "spoofchk": "off",
          "ipam": {
            "type": "whereabouts",
            "range": "51.1.1.0/24",
            "exclude": [
              "51.1.1.3/32",
              "51.1.1.4/32"
            ]
          }
        },
        {
          "type": "tuning",
          "sysctl": {
            "net.ipv4.conf.all.arp_announce": "2",
            "net.ipv4.conf.all.arp_filter": "1",
            "net.ipv4.conf.all.arp_ignore": "1",
            "net.ipv4.conf.all.rp_filter": "0",
            "net.ipv4.conf.all.accept_local": "1"
          },
          "mtu": 4220
        },
        {
          "type": "rdma"
        }
      ]
    }
```

### Verification

To verify that the RDMA CNI plugin is functioning correctly, run `rdma link show` inside the workload pod. The output should display only the RDMA device assigned to the pod in the `exclusive` mode, as shown below:

```bash
root@workload:/tmp# rdma link show
link ionic_0/1 state ACTIVE physical_state LINK_UP netdev net1
root@workload:/tmp#
```

In contrast, on a system configured in RDMA `shared` mode, the same command will list all available RDMA devices:

```bash
root@rccl-app-5f8b8dbddb-fgr6s:/tmp# rdma link show
link roceo3/1 state ACTIVE physical_state LINK_UP netdev net1
link rocep132s0/1 state ACTIVE physical_state LINK_UP
link rocep33s0f0/1 state ACTIVE physical_state LINK_UP
link rocep33s0f1/1 state ACTIVE physical_state LINK_UP
link ionic_0/1 state ACTIVE physical_state LINK_UP
root@rccl-app-5f8b8dbddb-fgr6s:/tmp#
```

## SBR (Source-Based Routing) CNI Plugin

The [SBR CNI plugin](https://github.com/k8snetworkplumbingwg/sbr-cni) enables Source-Based Routing (also known as policy-based routing) for secondary networks. This allows traffic originating from specific source IP addresses or interfaces to use designated routing tables and gateways, which is essential for multi-homed pods that need to route traffic through specific interfaces based on the source address.

### AMD Host Device + SBR Example

The SBR plugin can be used with the AMD Host Device CNI to enable proper source-based routing for secondary networks. Note that, for /31 point-to-point IPv4 networks where amd-host-device computes a gateway, it will pass the IP address and gateway information to SBR for correct routing configuration.

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: vf-amd-host-device-sbr-nad
  annotations:
    k8s.v1.cni.cncf.io/resourceName: amd.com/vnic
spec:
  config: '{
  "cniVersion": "0.3.1",
  "name": "vf-amd-host-device-sbr-nad",
  "plugins": [
    {
      "type": "amd-host-device"
    },
    {
      "type": "sbr"
    }
  ]
}'
```

### How SBR Works

When the SBR plugin is invoked:

1. It reads the IPAM result from the previous CNI plugin in the chain
2. For each IP address with a gateway, it creates:
   - A new routing table (using a unique table ID)
   - A default route in that table pointing to the specified gateway
   - An IP rule that directs packets with that source address to use the new routing table
3. This ensures that traffic originating from the secondary interface is routed through the correct gateway

### Verification

To verify SBR configuration inside a pod:

```bash
# Show routing tables
ip route show table all
```

```bash
# Example output showing routes in custom tables:
root@workload-app-647dc5f6fc-pvpw2:/tmp# ip route show table all
default via 192.168.4.9 dev net1 table 100
192.168.4.8/31 dev net1 table 100 proto kernel scope link src 192.168.4.8
default via 192.168.2.9 dev net2 table 102
192.168.2.8/31 dev net2 table 102 proto kernel scope link src 192.168.2.8
default via 192.168.1.9 dev net3 table 104
192.168.1.8/31 dev net3 table 104 proto kernel scope link src 192.168.1.8
default via 192.168.6.9 dev net4 table 106
192.168.6.8/31 dev net4 table 106 proto kernel scope link src 192.168.6.8
default via 192.168.5.9 dev net5 table 108
192.168.5.8/31 dev net5 table 108 proto kernel scope link src 192.168.5.8
default via 192.168.7.9 dev net6 table 111
192.168.7.8/31 dev net6 table 111 proto kernel scope link src 192.168.7.8
default via 192.168.8.9 dev net7 table 114
192.168.8.8/31 dev net7 table 114 proto kernel scope link src 192.168.8.8
default via 192.168.3.9 dev net8 table 116
192.168.3.8/31 dev net8 table 116 proto kernel scope link src 192.168.3.8
```

```bash
# Show IP rules
ip rule show
```

```bash
# Example output showing SBR rules:
root@workload-app-647dc5f6fc-pvpw2:/tmp# ip rule show
0:      from all lookup local
32758:  from 192.168.3.8 lookup 116
32759:  from 192.168.8.8 lookup 114
32760:  from 192.168.7.8 lookup 111
32761:  from 192.168.5.8 lookup 108
32762:  from 192.168.6.8 lookup 106
32763:  from 192.168.1.8 lookup 104
32764:  from 192.168.2.8 lookup 102
32765:  from 192.168.4.8 lookup 100
32766:  from all lookup main
32767:  from all lookup default
```

To ping a NIC interface between two pods, you can use the following command:

```bash
ping <IP_ADDRESS> -I <interface-name or ip-address>
```

```bash
root@workload-app-647dc5f6fc-pvpw2:/tmp# ping 192.168.1.14 -I 192.168.3.8
PING 192.168.1.14 (192.168.1.14) from 192.168.3.8 : 56(84) bytes of data.
64 bytes from 192.168.1.14: icmp_seq=1 ttl=61 time=0.167 ms
64 bytes from 192.168.1.14: icmp_seq=2 ttl=61 time=0.185 ms
^C
--- 192.168.1.14 ping statistics ---
2 packets transmitted, 2 received, 0% packet loss, time 1018ms
rtt min/avg/max/mdev = 0.167/0.176/0.185/0.009 ms
root@workload-app-647dc5f6fc-pvpw2:/tmp#
```
