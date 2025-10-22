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

