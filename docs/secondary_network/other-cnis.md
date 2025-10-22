# Alternative CNI Plugins

This document provides guidance on using different CNI plugins that have been tested with AMD Network Operator as alternatives to the AMD Host Device CNI plugin. These CNI plugins can be used with `NetworkAttachmentDefinition` to address specific networking use cases.

The AMD Network Operator supports multiple CNI plugins through NetworkAttachmentDefinition configurations, below are the few CNIs that we were tested:
- **SR-IOV CNI**: For high-performance networking using Virtual Functions (VFs)
- **RDMA CNI**: For RDMA workloads requiring network namespace isolation

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
      "cniVersion": "1.0.0",
      "name": "sriov-nad",
      "config":
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