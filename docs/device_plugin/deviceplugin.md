# Device Plugin & Node Labeller

## Configure Device Plugin and Node Labeller

To enable the Device Plugin and Node Labeller alongside the Network Operator, configure the fields under the `spec.devicePlugin` section in the NetworkConfig Custom Resource (CR):

```yaml
apiVersion: amd.com/v1alpha1
kind: NetworkConfig
metadata:
    name: example-networkconfig
spec:
    ...
    devicePlugin:
        ...
        # Enable the Node Labeller component (default: true)
        enableNodeLabeller: true

        # Specify the Node Labeller image (default: docker.io/rocm/k8s-network-node-labeller:v1.0.0)
        nodeLabellerImage: "docker.io/rocm/k8s-network-node-labeller:v1.0.0"

        # Node labeller image pull policy
        nodeLabellerImagePullPolicy: Always

        # Specify the Device Plugin image (default: docker.io/rocm/k8s-network-device-plugin:v1.0.0)
        devicePluginImage: "docker.io/rocm/k8s-network-device-plugin:v1.0.0"

        # Device plugin image pull policy
        devicePluginImagePullPolicy: Always
    ...
```

### Field Description

| Field Name                       | Description                                     |
|----------------------------------|-------------------------------------------------|
| **DevicePluginImage**            | Device plugin image                             |
| **DevicePluginImagePullPolicy**  | One of Always, Never, IfNotPresent.             |
| **NodeLabellerImage**            | Image to use for the Node Labeller              |
| **NodeLabellerImagePullPolicy**  | Image pull policy: Always, Never, IfNotPresent  |
| **EnableNodeLabeller**           | Enable or disable the Node Labeller (true/false)|
</br>

The `ImagePullPolicy` field defaults to `Always` if the image tag is `:latest`, or to `IfNotPresent` for other tags. This follows the default Kubernetes behavior for `ImagePullPolicy`.

Device Plugin and Node Labeller pods will start automatically after you update the NetworkConfig CR.

```bash
#kubectl get pods -n kube-amd-network
NAME                                                              READY   STATUS              RESTARTS   AGE
amd-network-operator-kmm-controller-8558dd8554-pnklg              1/1     Running             0          82s
amd-network-operator-kmm-webhook-server-6d54d5556-wn6dr           1/1     Running             0          82s
amd-network-operator-multus-multus-zm75t                          1/1     Running             0          82s
amd-network-operator-network-operator-charts-controller-ma64rjp   1/1     Running             0          82s
amd-network-operator-node-feature-discovery-gc-77d6d6449c-t85rz   1/1     Running             0          82s
amd-network-operator-node-feature-discovery-master-869f4bbprrhw   1/1     Running             0          82s
amd-network-operator-node-feature-discovery-worker-vbcxx          1/1     Running             0          82s
test-networkconfig-device-plugin-l89f9                            1/1     Running             0          53s
test-networkconfig-node-labeller-kthdz                            1/1     Running             0          53s
```
