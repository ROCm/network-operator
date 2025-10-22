# AMD Network Operator
Network Operator simplifies the use of AMD AINICs in Kubernetes environments. It manages all networking components required to enable RDMA workloads within a Kubernetes cluster.

- AMD networking drivers - provide support for AMD AINICs and enable advanced features such as RDMA, SR-IOV, and hardware acceleration for high-performance workloads.
- Kubernetes device plugins - expose AINIC hardware capabilities to containers, allowing workloads to directly access high-speed network interfaces with minimal overhead.
- Kubernetes secondary networks - integrate with CNI plugins (like Multus) to provide dedicated network paths for data-intensive or latency-sensitive applications, ensuring optimal performance and isolation.

## Components
* AMD Network Operator Controller
* Kernel Module Management Operator
* Node Feature Discovery Operator
* K8s Network Device Plugin
* K8s Network Node Labeller
* Device Metrics Exporter
* Multus CNI
* CNI Plugins

## Installation
### Prerequisites
* Kubernetes v1.29.0+
* Helm v3.2.0+
* `kubectl` CLI tool configured to access your cluster
* Networking kernel modules installed on all nodes (modprobe br_netfilter vxlan)

1. Install Flannel CNI (if not present) for inter-node communication between nodes
```bash
 kubectl apply -f https://raw.githubusercontent.com/flannel-io/flannel/master/Documentation/kube-flannel.yml
```

2. Install the Network Operator using the Helm bundle
```bash
helm install amd-network-operator network-operator-helm-k8s-v1.0.0.tgz \
  -n kube-amd-network \
  --create-namespace \
  --set kmm.enabled=false
```
This command deploys the following components: Controller, Node Feature Discovery Operator and Multus CNI plugin

3. Create CR to run Device Plugin, Exporter and CNI Plugins

If you are pulling images from a private registry, create a Kubernetes secret and update `networkconfig.yaml` to reference it.

***Create the Docker registry secret:***
```bash
kubectl create secret docker-registry my-secret \
  --docker-server=docker.io \
  --docker-username=<username> \
  --docker-password=<password> \
  -n kube-amd-network
```

***Apply your network configuration:***
```bash
kubectl apply -f example/networkconfig.yaml
```
After the Device Plugin is up and running, it will begin discovering NIC devices and reporting them to the kubelet.

4. Create a Network Attachment Definition to assign requested device to a workload
```bash
kubectl apply -f example/amd-host-device.nad
```
This step defines a secondary network using a NetworkAttachmentDefinition, which ensures the requested NIC or vNIC device is assigned to the workload pod via Multus.

5. Create a workload requesting for a nic/vnic
```bash
kubectl apply -f example/workload.yaml
```

6. Exec into the workload pods and run IB and RCCL tests between the nodes. 
```bash
root@app:/tmp# ib_write_bw -d roce_ai1 -i 1 -n 1000 -F -a -x 1 -q 1
root@app:/tmp# /tmp/vf_rccl_run.sh
```

## Image URLs
* Network Operator: `docker pull amdpsdo/network-operator:network-operator-0.0.1-2`
* K8s Network Device Plugin `docker pull amdpsdo/k8s-network-device-plugin:v1.0.0-beta.0`
* Device Metrics Exporter: `docker pull amdpsdo/device-metrics-exporter:exporter-0.0.1-139`
* CNI Plugins: `docker pull amdpsdo/cni-plugins:v1.0.0-beta.0`
* RCCL & IB Workload: `docker pull docker.io/rocm/roce-workload:ubuntu24_rocm7_rccl-J13A-1_anp-v1.1.0-4D_ainic-1.117.1-a-63`


## Additional Installation Instructions

### Whereabouts IPAM Plugin

1. IPAM plugin used to allocate and manage IP addresses for pods in a cluster in a way that avoids IP conflicts.
2. Clone the whereabouts repository and apply necessary k8s manifests:
```bash
    git clone https://github.com/k8snetworkplumbingwg/whereabouts && cd whereabouts
    kubectl apply \
        -f doc/crds/daemonset-install.yaml \
        -f doc/crds/whereabouts.cni.cncf.io_ippools.yaml \
        -f doc/crds/whereabouts.cni.cncf.io_overlappingrangeipreservations.yaml
```

### RDMA CNI Plugin

1. RDMA CNI plugin allows network namespace isolation for RDMA workloads in a containerized environment.
2. Set RDMA subsystem namespace awareness mode to `exclusive` during OS boot on all the worker nodes.
```bash
    echo "options ib_core netns_mode=0" >> /etc/modprobe.d/ib_core.conf
```
3. Clone the RDMA CNI repository and apply necessary k8s manifests:
```bash
    git clone https://github.com/k8snetworkplumbingwg/rdma-cni && cd rdma-cni
    kubectl apply \
      -f ./deployment/rdma-cni-daemonset.yaml
```
4. To verify that the RDMA CNI plugin is functioning correctly, run `rdma link show` inside the workload pod. The output should display only the RDMA device assigned to the pod in the `exclusive` mode, as shown below:
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

Please refer to [cni](docs/cni.md) for usage examples of these CNI plugins.

## Troubleshooting
To run the tool and troubleshoot issues, execute the following command:
```bash
./tools/techsupport_dump.sh -w -o yaml <node-name>
```
Please refer to the troubleshooting page [here](docs/troubleshooting.md) for more details.

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
| controllerManager.manager.image.repository | string | `"registry.test.pensando.io:5000/amd-network-operator"` | AMD Network operator controller manager image repository |
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
| kmm.controller.manager.env.relatedImageSign | string | `"registry.test.pensando.io:5000/kernel-module-management-signimage:latest"` | KMM signer image for signing driver image's kernel module with given key pairs within cluster |
| kmm.controller.manager.env.relatedImageSignPullSecret | string | `""` | Image pull secret name for pulling KMM signer image if registry needs credential to pull image |
| kmm.controller.manager.env.relatedImageWorker | string | `"registry.test.pensando.io:5000/kernel-module-management-worker:latest"` | KMM worker image for loading / unloading driver kernel module on worker nodes |
| kmm.controller.manager.env.relatedImageWorkerPullSecret | string | `""` | Image pull secret name for pulling KMM worker image if registry needs credential to pull image |
| kmm.controller.manager.image.repository | string | `"registry.test.pensando.io:5000/kernel-module-management-operator"` | KMM controller manager image repository |
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
| kmm.webhookServer.webhookServer.image.repository | string | `"registry.test.pensando.io:5000/kernel-module-management-webhook-server"` | KMM webhook image repository |
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

