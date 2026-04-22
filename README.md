# AMD Network Operator

The AMD Network Operator simplifies the deployment and management of AMD AINIC networking components in Kubernetes environments. It provides a unified solution for enabling high-performance networking capabilities including RDMA, SR-IOV, and hardware acceleration.

## Overview

The AMD Network Operator manages all networking components required to enable advanced network workloads within a Kubernetes cluster:

- **AMD networking drivers** - Support for AMD AINICs with advanced features
- **Kubernetes device plugin** - Expose AINIC hardware capabilities to containers
- **Secondary networks** - Integration with CNI plugins for dedicated network paths

For detailed component information, see [Component Overview](docs/overview.md).

## License

The AMD Network Operator is licensed under the [Apache License 2.0](LICENSE).

