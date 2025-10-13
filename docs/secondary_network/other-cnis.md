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

The RDMA CNI plugin provides network namespace isolation for RDMA workloads in containerized environments. It must be chained with other CNI plugins that handle the actual interface movement.

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