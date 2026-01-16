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

# network-operator-charts

![Version: v1.0.0](https://img.shields.io/badge/Version-v1.0.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: dev](https://img.shields.io/badge/AppVersion-dev-informational?style=flat-square)

AMD Network Operator simplifies the deployment and management of AMD AINICs within Kubernetes clusters.

**Homepage:** <https://github.com/ROCm/network-operator>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Sundara Gurunathan<Sundaramurthy.Gurunathan@amd.com> |  |  |
| Yuvarani Shankar<Yuvarani.Shankar@amd.com> |  |  |
| Shrey Ajmera<Shrey.Ajmera@amd.com> |  |  |

## Source Code

* <https://github.com/ROCm/network-operator>

## Requirements

Kubernetes: `>= 1.29.0-0`

| Repository | Name | Version |
|------------|------|---------|
| file://./charts/kmm | kmm | v1.0.0 |
| file://./charts/multus | multus | v1.0.0 |
| https://kubernetes-sigs.github.io/node-feature-discovery/charts | node-feature-discovery | v0.16.1 |

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| controllerManager.affinity | object | `{"nodeAffinity":{"preferredDuringSchedulingIgnoredDuringExecution":[{"preference":{"matchExpressions":[{"key":"node-role.kubernetes.io/control-plane","operator":"Exists"}]},"weight":1}]}}` | Deployment affinity configs for controller manager |
| controllerManager.manager.image.repository | string | `"docker.io/rocm/amd-network-operator"` | AMD Network operator controller manager image repository |
| controllerManager.manager.image.tag | string | `"dev"` | AMD Network operator controller manager image tag |
| controllerManager.manager.imagePullPolicy | string | `"Always"` | Image pull policy for AMD Network operator controller manager pod |
| controllerManager.manager.imagePullSecrets | string | `""` | Image pull secret name for pulling AMD Network operator controller manager image if registry needs credential to pull image |
| controllerManager.nodeSelector | object | `{}` | Node selector for AMD Network operator controller manager deployment |
| installdefaultNFDRule | bool | `true` | Default NFD rule will detect amd network based on pci vendor ID |
| kmm.enabled | bool | `true` | Set to true/false to enable/disable the installation of kernel module management (KMM) operator |
| multus.enabled | bool | `true` | Set to true/false to enable/disable the installation of multus cni |
| node-feature-discovery.enabled | bool | `true` | Set to true/false to enable/disable the installation of node feature discovery (NFD) operator |
| upgradeCRD | bool | `true` | CRD will be patched as pre-upgrade/pre-rollback hook when doing helm upgrade/rollback to current helm chart |
| kmm.controller.affinity | object | `{"nodeAffinity":{"preferredDuringSchedulingIgnoredDuringExecution":[{"preference":{"matchExpressions":[{"key":"node-role.kubernetes.io/control-plane","operator":"Exists"}]},"weight":1}]}}` | Affinity for the KMM controller manager deployment |
| kmm.controller.manager.args[0] | string | `"--config=controller_config.yaml"` |  |
| kmm.controller.manager.containerSecurityContext.allowPrivilegeEscalation | bool | `false` |  |
| kmm.controller.manager.env.relatedImageBuild | string | `"gcr.io/kaniko-project/executor:v1.23.2"` | KMM kaniko builder image for building driver image within cluster |
| kmm.controller.manager.env.relatedImageBuildPullSecret | string | `""` | Image pull secret name for pulling KMM kaniko builder image if registry needs credential to pull image |
| kmm.controller.manager.env.relatedImageSign | string | `"docker.io/rocm/kernel-module-management-signimage:latest"` | KMM signer image for signing driver image's kernel module with given key pairs within cluster |
| kmm.controller.manager.env.relatedImageSignPullSecret | string | `""` | Image pull secret name for pulling KMM signer image if registry needs credential to pull image |
| kmm.controller.manager.env.relatedImageWorker | string | `"docker.io/rocm/kernel-module-management-worker:latest"` | KMM worker image for loading / unloading driver kernel module on worker nodes |
| kmm.controller.manager.env.relatedImageWorkerPullSecret | string | `""` | Image pull secret name for pulling KMM worker image if registry needs credential to pull image |
| kmm.controller.manager.image.repository | string | `"docker.io/rocm/kernel-module-management-operator"` | KMM controller manager image repository |
| kmm.controller.manager.image.tag | string | `"latest"` | KMM controller manager image tag |
| kmm.controller.manager.imagePullPolicy | string | `"Always"` | Image pull policy for KMM controller manager pod |
| kmm.controller.manager.imagePullSecrets | string | `""` | Image pull secret name for pulling KMM controller manager image if registry needs credential to pull image |
| kmm.controller.manager.resources.limits.cpu | string | `"500m"` |  |
| kmm.controller.manager.resources.limits.memory | string | `"384Mi"` |  |
| kmm.controller.manager.resources.requests.cpu | string | `"10m"` |  |
| kmm.controller.manager.resources.requests.memory | string | `"64Mi"` |  |
| kmm.controller.manager.tolerations[0].effect | string | `"NoSchedule"` |  |
| kmm.controller.manager.tolerations[0].key | string | `"node-role.kubernetes.io/master"` |  |
| kmm.controller.manager.tolerations[0].operator | string | `"Equal"` |  |
| kmm.controller.manager.tolerations[0].value | string | `""` |  |
| kmm.controller.manager.tolerations[1].effect | string | `"NoSchedule"` |  |
| kmm.controller.manager.tolerations[1].key | string | `"node-role.kubernetes.io/control-plane"` |  |
| kmm.controller.manager.tolerations[1].operator | string | `"Equal"` |  |
| kmm.controller.manager.tolerations[1].value | string | `""` |  |
| kmm.controller.nodeSelector | object | `{}` | Node selector for the KMM controller manager deployment |
| kmm.controller.replicas | int | `1` |  |
| kmm.controller.serviceAccount.annotations | object | `{}` |  |
| kmm.controllerMetricsService.ports[0].name | string | `"https"` |  |
| kmm.controllerMetricsService.ports[0].port | int | `8443` |  |
| kmm.controllerMetricsService.ports[0].protocol | string | `"TCP"` |  |
| kmm.controllerMetricsService.ports[0].targetPort | string | `"https"` |  |
| kmm.controllerMetricsService.type | string | `"ClusterIP"` |  |
| kmm.kubernetesClusterDomain | string | `"cluster.local"` |  |
| kmm.managerConfig.controllerConfigYaml | string | `"healthProbeBindAddress: :8081\nwebhookPort: 9443\nleaderElection:\n  enabled: true\n  resourceID: kmm.sigs.x-k8s.io\nmetrics:\n  enableAuthnAuthz: true\n  bindAddress: 0.0.0.0:8443\n  secureServing: true\nworker:\n  runAsUser: 0\n  seLinuxType: spc_t\n  firmwareHostPath: /var/lib/firmware"` |  |
| kmm.webhookServer.affinity | object | `{"nodeAffinity":{"preferredDuringSchedulingIgnoredDuringExecution":[{"preference":{"matchExpressions":[{"key":"node-role.kubernetes.io/control-plane","operator":"Exists"}]},"weight":1}]}}` | KMM webhook's deployment affinity configs |
| kmm.webhookServer.nodeSelector | object | `{}` | KMM webhook's deployment node selector |
| kmm.webhookServer.replicas | int | `1` |  |
| kmm.webhookServer.webhookServer.args[0] | string | `"--config=controller_config.yaml"` |  |
| kmm.webhookServer.webhookServer.args[1] | string | `"--enable-module"` |  |
| kmm.webhookServer.webhookServer.args[2] | string | `"--enable-namespace"` |  |
| kmm.webhookServer.webhookServer.args[3] | string | `"--enable-preflightvalidation"` |  |
| kmm.webhookServer.webhookServer.containerSecurityContext.allowPrivilegeEscalation | bool | `false` |  |
| kmm.webhookServer.webhookServer.image.repository | string | `"docker.io/rocm/kernel-module-management-webhook-server"` | KMM webhook image repository |
| kmm.webhookServer.webhookServer.image.tag | string | `"latest"` | KMM webhook image tag |
| kmm.webhookServer.webhookServer.imagePullPolicy | string | `"Always"` | Image pull policy for KMM webhook pod |
| kmm.webhookServer.webhookServer.imagePullSecrets | string | `""` | Image pull secret name for pulling KMM webhook image if registry needs credential to pull image |
| kmm.webhookServer.webhookServer.resources.limits.cpu | string | `"500m"` |  |
| kmm.webhookServer.webhookServer.resources.limits.memory | string | `"384Mi"` |  |
| kmm.webhookServer.webhookServer.resources.requests.cpu | string | `"10m"` |  |
| kmm.webhookServer.webhookServer.resources.requests.memory | string | `"64Mi"` |  |
| kmm.webhookServer.webhookServer.tolerations[0].effect | string | `"NoSchedule"` |  |
| kmm.webhookServer.webhookServer.tolerations[0].key | string | `"node-role.kubernetes.io/master"` |  |
| kmm.webhookServer.webhookServer.tolerations[0].operator | string | `"Equal"` |  |
| kmm.webhookServer.webhookServer.tolerations[0].value | string | `""` |  |
| kmm.webhookServer.webhookServer.tolerations[1].effect | string | `"NoSchedule"` |  |
| kmm.webhookServer.webhookServer.tolerations[1].key | string | `"node-role.kubernetes.io/control-plane"` |  |
| kmm.webhookServer.webhookServer.tolerations[1].operator | string | `"Equal"` |  |
| kmm.webhookServer.webhookServer.tolerations[1].value | string | `""` |  |
| kmm.webhookService.ports[0].port | int | `443` |  |
| kmm.webhookService.ports[0].protocol | string | `"TCP"` |  |
| kmm.webhookService.ports[0].targetPort | int | `9443` |  |
| kmm.webhookService.type | string | `"ClusterIP"` |  |
| multus.cniBinDir | string | `"/opt/cni/bin"` |  |
| multus.image.pullPolicy | string | `"IfNotPresent"` |  |
| multus.image.repository | string | `"ghcr.io/k8snetworkplumbingwg/multus-cni"` |  |
| multus.image.tag | string | `"v4.2.2"` |  |
| multus.multusConfig.confDir | string | `"/etc/cni/net.d"` |  |
| multus.multusConfig.fileName | string | `"00-multus.conf"` |  |
| multus.rbac.create | bool | `true` |  |
| multus.serviceAccountName | string | `"multus"` |  |

