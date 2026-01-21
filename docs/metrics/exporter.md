# Metrics Exporter

## Configure Metrics Exporter

To enable the Metrics Exporter alongside the Network Operator, configure the fields under the `spec.metricsExporter` section in the NetworkConfig Custom Resource (CR):

```yaml
apiVersion: amd.com/v1alpha1
kind: NetworkConfig
metadata:
    name: example-networkconfig
spec:
    ...
    metricsExporter:
        # Enable the Metrics Exporter component (default: false)
        enable: true

        # Specify the Metrics Exporter image
        image: "docker.io/rocm/k8s-network-metrics-exporter:nic-v1.0.0"

        # Image pull policy (default: IfNotPresent, or Always if tag is :latest)
        imagePullPolicy: "IfNotPresent"

        # Port for metrics endpoint (default: 5001)
        port: 5001

        # Service type for metrics access (default: ClusterIP)
        serviceType: "NodePort"

        # NodePort for external access (should be between 30000-32767)
        # Works with serviceType: "NodePort"
        nodePort: 32501

        # Use host networking (default: true)
        hostNetwork: true
        ...
    ...
```

### Field Description

| Field Name              | Description                                                  | Default Value      |
|-------------------------|--------------------------------------------------------------|--------------------|
| **enable**              | Enable or disable the Metrics Exporter (true/false)          | false              |
| **image**               | Container image to use for the Metrics Exporter              | -                  |
| **imagePullPolicy**     | Image pull policy: Always, Never, IfNotPresent               | IfNotPresent       |
| **imageRegistrySecret** | Secret for pulling images from private registries            | -                  |
| **port**                | Internal port for metrics endpoint                           | 5001               |
| **serviceType**         | Service type for metrics access: ClusterIP, NodePort         | ClusterIP          |
| **nodePort**            | External port for NodePort service (30000-32767)             |                    |
| **hostNetwork**         | Enable host networking for the exporter pods                 | true               |
| **selector**            | Node selector for fine-grained pod placement                 | -                  |
| **tolerations**         | Pod tolerations for scheduling                               | -                  |
| **upgradePolicy**       | DaemonSet upgrade strategy configuration                     | -                  |
| **config**              | Configmap containing exporter config.json                    | -                  |
| **rbacConfig**          | Optional RBAC proxy configuration                            | -                  |

**Note:**

- The `ImagePullPolicy` field defaults to `Always` if the image tag is `:latest`, or to `IfNotPresent` for other tags. This follows the default Kubernetes behavior for `ImagePullPolicy`.
- For the exporter pod to be able to fetch all metrics, we recommend running the pod with `hostNetwork` set to `true`, which is the default behavior.

The Metrics Exporter is deployed as a DaemonSet, which means one pod runs on each node that matches the specified `selector`.

### Node Selection Behavior

The NetworkConfig CR has a global `spec.selector` field that controls deployment of all operands (like device plugin, node labeller and metrics exporter) under the NetworkConfig. However, you can override this with component-specific selectors:

- **Global selector**: Controls all operands when no component-specific selector is set
- **Component selector**: When `metricsExporter.selector` is specified, it provides fine-grained control and overrides the global NetworkConfig selector for the Metrics Exporter component only

## Deployment

Metrics Exporter pods will start automatically after you update the NetworkConfig CR with the metrics exporter configuration:

```bash
kubectl get pods -n kube-amd-network
NAME                                                              READY   STATUS    RESTARTS   AGE
amd-network-operator-kmm-controller-8558dd8554-pnklg              1/1     Running   0          23s
amd-network-operator-kmm-webhook-server-6d54d5556-wn6dr           1/1     Running   0          23s
amd-network-operator-multus-multus-zm75t                          1/1     Running   0          23s
amd-network-operator-network-operator-charts-controller-ma64rjp   1/1     Running   0          23s
amd-network-operator-node-feature-discovery-gc-77d6d6449c-t85rz   1/1     Running   0          23s
amd-network-operator-node-feature-discovery-master-869f4bbprrhw   1/1     Running   0          23s
amd-network-operator-node-feature-discovery-worker-vbcxx          1/1     Running   0          23s
test-networkconfig-device-plugin-l89f9                            1/1     Running   0          8s
test-networkconfig-metrics-exporter-htdew                         1/1     Running   0          8s
```

## Accessing Metrics

The Metrics Exporter creates a Kubernetes service to expose metrics. For more information about Kubernetes services, see the [official kubernetes documentation](https://kubernetes.io/docs/concepts/services-networking/service/).

### From Within the Cluster (ClusterIP)

Access metrics from within the cluster using the ClusterIP:

```bash
# Access metrics via ClusterIP
curl http://<cluster-ip>:<port>/metrics
```

**Note**: The `<cluster-ip>` can be obtained by running `kubectl get svc -n kube-amd-network` and finding the metrics exporter service,  the cluster IP is listed in the CLUSTER-IP column. The `<port>` is configured in your [NetworkConfig](#configure-metrics-exporter) (default: 5001).

### From Outside the Cluster (NodePort)

To access metrics from outside the cluster, you must enable NodePort by setting `serviceType: "NodePort"` in your [NetworkConfig](#configure-metrics-exporter).

```bash
# Access metrics via any node's IP and NodePort
curl http://<node-ip>:<node-port>/metrics
```

**Note**: The `<node-ip>` can be the IP address of any node in your cluster. The `<node-port>` is either auto-assigned by Kubernetes or explicitly set via the `nodePort` field in your [NetworkConfig](#configure-metrics-exporter).

## Advanced Configuration

To customize metrics fields, labels and other advanced setting, create a ConfigMap with the desired values and reference it in your [NetworkConfig](#configure-metrics-exporter).

```bash
kubectl apply -f path/to/your/configmap.yaml
```

An example ConfigMap is available here: [configmap.yaml](https://github.com/ROCm/device-metrics-exporter/blob/main/example/configmap.yaml)

**Note:** When the Metrics Exporter is deployed through the Network Operator, GPU metrics are automatically disabled via the `monitor-gpu=false` argument (not user-configurable). This means:

- Only NIC-related metrics are exported
- Including GPU fields in your ConfigMap will not enable GPU metrics collection  
- The example ConfigMap is a generic configuration that works with both GPU and Network operators - each operator exports only its relevant metrics
