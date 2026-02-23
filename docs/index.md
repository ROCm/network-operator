## Introduction

AMD Network Operator simplifies the use of AMD AINICs in Kubernetes environments. It manages all networking components required to enable RDMA workloads within a Kubernetes cluster.

## Features

* Automated driver installation and management
* Comprehensive metrics collection and export
* Easy deployment of AMD AI NIC device plugin
* Automated worker node labeling on Kubernetes enabled nodes
* Efficient resource allocation for containers
* AI NIC health monitoring and troubleshooting  

## Compatibility

### **Supported Hardware**

| Hardware | Status |
|-----------|---------|
| AMD Pensando™ Pollara AI NIC | ✅ Supported |

### OS & Platform Support Matrix

Below is a list of operating systems and Kubernetes versions validated with the AMD Network Operator and Metrics Exporter.  
Additional versions will be added in future releases.

| Operating System | Kubernetes Versions |
|------------------|---------------------|
| Ubuntu 22.04 LTS | 1.29 – 1.34 |
| Ubuntu 24.04 LTS | 1.29 – 1.34 |

### Software Version Compatibility Matrix

| Network Operator | AINIC Firmware | Supported NICs |
|------------------|----------------|----------------|
| v1.0.0           | 1.117.1-a-63   | Pollara 400    |
| v1.0.1           | 1.117.1-a-63   | Pollara 400    |
| v1.1.0           | 1.117.5-a-56   | Pollara 400    |

## Prerequisites

* Kubernetes v1.29.0+
* Helm v3.2.0+
* `kubectl` CLI tool configured to access your cluster
* Networking kernel modules installed on all nodes (`modprobe br_netfilter vxlan`)

## Support

For bugs and feature requests, please file an issue on our [GitHub Issues](https://github.com/ROCm/network-operator/issues) page.
