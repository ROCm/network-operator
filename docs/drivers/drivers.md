# Driver Management Guide

This guide explains how to manage AMD AI NIC drivers using the AMD Network Operator on Kubernetes clusters.

## Prerequisites

Before installing the AMD AI NIC driver:

1. Ensure the AMD Network Operator and its dependencies are successfully deployed
2. Have cluster admin permissions
3. Have access to an image registry for driver images (if trying to install out-of-tree driver by operator)

## Installation Steps

### Inbox or Pre-installed AI NIC driver

If you want to use inbox / pre-installed AI NIC driver, use `lsmod` command to verify they are already loaded on your worker node. For example, if the ionic driver was already loaded on your worker node the `lsmod` would be:

```bash
$ lsmod | grep ionic
ionic_rdma            233472  0
ionic                 258048  1 ionic_rdma
ib_peer_mem            20480  1 ionic_rdma
ib_uverbs             184320  3 ib_peer_mem,ionic_rdma,rdma_ucm
ib_core               507904  8 rdma_cm,rpcrdma,ionic_rdma,iw_cm,ib_iser,rdma_ucm,ib_uverbs,ib_cm
```

When you create the `NetworkConfig` custom resource, you don't need to use the driver related fields:

```yaml
spec:
  driver:
    enable: false
```

### Out-of-tree driver installation by AMD Network Operator

To install the ionic driver by using AMD Network Operator, please prepare an image registry to store the compiled driver images, then specify corresponding fields in the driver spec of `NetworkConfig`. 

```{note}
Some Operating System may contain an inbox `ionic` kernel module. That inbox kernel module could be old versions and affect the installation of desired version out-of-tree kernel module. To blacklist the inbox `ionic` driver please specify `spec.driver.blacklist` as true. After that the worker nodes may need to these to apply the blacklist and avoid the usage of inbox `ionic` kernel module:

* sudo update-initramfs -u
* sudo reboot
```

For example:

if you are using secure registry and requires a credential to get image pull / push access, please prepare the credential as Kubernetes secret:

```bash
# ignore --docker-server if you are using DockerHub
kubectl create secret docker-registry mysecret \
  -n kube-amd-network \
  --docker-server=registry.example.com \
  --docker-username=xxx \
  --docker-password=xxx
```

then specify the information in `NetworkConfig`:

```yaml
spec:
 driver:
   enable: true
   # DO NOT input the image tag, operator will automatically handle the image tag
   image: registry.example.com/username/amdainic_kmods
   # (Optional) Specify the credential for your private registry if it requires credential to get pull/push access
   # you can create the docker-registry type secret by running command like:
   # kubectl create secret docker-registry my-secret -n kube-amd-network --docker-username=xxx --docker-password=xxx
   # Make sure you created the secret within the namespace that KMM operator is running
   imageRegistrySecret:
     name: my-secret
   version: 1.117.1-a-42
```

if you are using insecure image registry, please specify the TLS configs, for example:

```yaml
spec:
 driver:
   enable: true
   # DO NOT input the image tag, operator will automatically handle the image tag
   image: insecure.registry.io:5000/username/amdainic_kmods
   imageRegistryTLS:
     insecure: true
     insecureSkipTLSVerify: true
   version: 1.117.1-a-42
```

## Driver installation verification

If you successfully installed ionic driver on the worker nodes by AMD Network Operator, you should be able to see the KMM operator labeled the node with its driver ready label and driver version label. For example for a `NetworkConfig` named `test-networkconfig` in namespace `kube-amd-network`, it will show:

```bash
$ kubectl get node -oyaml | grep kmm
      kmm.node.kubernetes.io/kube-amd-network.test-networkconfig.ready: ""
      kmm.node.kubernetes.io/version-module.kube-amd-network.test-networkconfig: 1.117.1-a-42
```

Once the driver is loaded, the operand pods should be ready as well, for example the device plugin pod should be in ready state on that node and start to advertising the resource:

```bash
$ kubectl get pods -n kube-amd-network | grep device-plugin
test-networkconfig-device-plugin-r827t                            1/1     Running   0          22h

$ kubectl get node -oyaml | grep amd.com
      amd.com/vnic: "8"
      amd.com/vnic: "8"
```

## Driver uninstallation

If you use AMD Network Operator to install the ionic driver, you can uninstall the driver kernel modules by simply deleting the `NetworkConfig` custom resource. The deletion operator will wait for KMM Operator to unload ionic kernel modules on all selected worker nodes then finally remove the custom resource.

Please make sure there is no workload actively using the ionic kernel module before starting the driver uninstallation.

```bash
kubectl delete networkconfigs -A --all
```