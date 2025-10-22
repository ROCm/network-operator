# Custom Resource Guide

### Prerequsite 

AMD NIC drivers must be either included in the OS (inbox) or pre-installed on the worker node. Driver installation through the network operator is not supported currently.

### 1. Create DeviceConfig Resource

By deploying the following custom resource, the operator will directly deploy the device plugin, node labeller, metrics exporter, and CNI plugins on all worker nodes with AMD NICs.

```yaml
apiVersion: amd.com/v1alpha1
kind: NetworkConfig
metadata:
  name: test-networkconfig
  # namespace where AMD Network Operator is running
  namespace: kube-amd-network
spec:
  # Driver config
  driver:
    enable: true
    image: registry.example.com/username/amdainic_kmods
    imageRegistrySecret:
      name: my-secret
    version: 1.117.1-a-42

  # Device plugin and Node labeller config
  devicePlugin:
    enableNodeLabeller: True
    nodeLabellerImage: docker.io/rocm/k8s-network-node-labeller:v1.0.0
    devicePluginImage: docker.io/rocm/k8s-network-device-plugin:v1.0.0

  # Metrics exporter config
  metricsExporter:
    enable: True
    port: 5001
    serviceType: "NodePort"
    nodePort: 32500
    hostNetwork: true
    image: docker.io/rocm/device-metrics-exporter:nic-v1.0.0
  
  # Secondary network config
  secondaryNetwork:
    cniPlugins:
      enable: True
      image: docker.io/rocm/k8s-cni-plugins:v1.0.0
  
  # Specify the node to be managed by this NetworkConfig Custom Resource
  selector:
    feature.node.kubernetes.io/amd-nic: "true"
```

Refer to this [NetworkConfig CR example](networkconfig-full.md) for a comprehensive list of available configuration options.

#### Configuration Reference

To list existing `NetworkConfig` resources, run `kubectl get networkconfigs -A`

To check the full spec of `NetworkConfig` definition, run `kubectl get crds networkconfigs.amd.com -oyaml`

#### `metadata` Parameters

| Parameter | Description |
|-----------|-------------|
| `name` | Unique identifier for the resource |
| `namespace` | Namespace where the operator is running |

#### `spec.driver` Parameters
| Parameter | Description | Default |
|-----------|-------------|---------|
| `enable` | Enable/disable driver installation | `true` |
| `image` | Image URL to pull/push kernel modules images | |
| `version` | Driver version for source code builds | |
| `imageRegistrySecret.name` | Registry credentials secret for driver image | |
| `imageRegistryTLS.insecure` | Use plain HTTP for registry access | `false` |
| `imageRegistryTLS.insecureSkipTLSVerify` | Skip TLS certificate validation | `false` |

#### `spec.devicePlugin` Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `devicePluginImage` | AMD Network device plugin image | `docker.io/rocm/k8s-network-device-plugin:v1.0.0` |
| `nodeLabellerImage` | Node labeller image | `docker.io/rocm/k8s-network-node-labeller:v1.0.0` |
| `imageRegistrySecret.name` | Name of registry credentials secret<br> to pull device plugin / node labeller image | |
| `enableNodeLabeller` | enable / disable node labeller | `true` |

#### `spec.metricsExporter` Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `enable` | Enable/disable metrics exporter | `false` |
| `imageRegistrySecret.name` | Name of registry credentials secret<br> to pull metrics exporter image | |
| `serviceType` | Service type for metrics endpoint <br>Options: "ClusterIP" or "NodePort" | `ClusterIP` |
| `port` | clsuter IP's internal service port<br> for reaching the metrics endpoint | `5001` |
| `nodePort` | Port number when using NodePort service type | automatically assigned |
| `selector` | select which nodes to enable metrics exporter | same as `spec.selector` |

#### `spec.secondaryNetwork` Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `cniPlugins.enable` | Enable/disable CNI plugins | `false` |
| `cniPlugins.image` | CNI plugins image | `docker.io/rocm/cni-plugins:v1.0.0`|
| `cniPlugins.imageRegistrySecret.name` | Name of registry credentials secret<br> to pull metrics exporter image | |

#### `spec.selector` Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `selector` | Labels to select nodes for driver installation | `feature.node.kubernetes.io/amd-nic: "true"` |

### Registry Secret Configuration

If you're using a private container registry, create a Docker registry secret before deploying to supply the credentials needed to access the registry:```

```bash
kubectl create secret docker-registry mysecret \
  -n kube-amd-network \
  --docker-server=registry.example.com \
  --docker-username=xxx \
  --docker-password=xxx
```

If you are using DockerHub to host images, you don't need to specify the ```--docker-server``` parameter when creating the secret.

### 2. Monitor Installation Status

Check the deployment status:

```bash
kubectl get networkconfigs test-networkconfig -n kube-amd-network -o yaml
```

Example status output:

```yaml
status:
  conditions:
  - lastTransitionTime: "2025-08-28T23:18:14Z"
    message: ""
    reason: OperatorReady
    status: "True"
    type: Ready
  configManager: {}
  devicePlugin:
    availableNumber: 1              # Nodes with device plugin running
    desiredNumber: 1                # Target number of nodes
    nodesMatchingSelectorNumber: 1  # Nodes matching selector
  driver: {}
  metricsExporter:
    availableNumber: 1
    desiredNumber: 1
    nodesMatchingSelectorNumber: 1
  nodeModuleStatus:
    dp-ainicop-node1: {}            # Node name
  observedGeneration: 1
```

### Custom Resource Installation Validation

After applying configuration:

- Check NetworkConfig status:

```bash
kubectl get networkconfig test-networkconfig -n kube-amd-network -o yaml
```

- Check metrics endpoint (if enabled):

```bash
# For Metrics Exporter service type set to NodePort
kubectl get service test-networkconfig-metrics-exporter -n kube-amd-network -o yaml
curl http://<node-ip>:<nodePort>/metrics
```

- Verify worker node labels:

```bash
kubectl get nodes -l feature.node.kubernetes.io/amd-nic=true
```



