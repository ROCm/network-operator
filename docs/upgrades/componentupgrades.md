# Upgrading Components of Network Operator (Device Plugin, Node labeller, Metrics Exporter and CNI Plugins)

This guide outlines the steps to upgrade the Device Plugin, Node labeller, Metrics Exporter and CNI Plugins Daemonsets managed by the AMD Network Operator on a Kubernetes cluster.

These components need a upgrade policy to be mentioned to decide how the daemonset upgrade will be done.

-> DevicePlugin and Nodelabeller have a common UpgradePolicy Spec in DevicePlugin Spec

-> Metrics Exporter has its own UpgradePolicy Spec in Metrics Exporter Spec

-> CNI Plugins has its own UpgradePolicy Spec in CNI Plugins Spec

-> `UpgradePolicy` has 2 fields, `UpgradeStrategy` (string) and `MaxUnavailable` (int)

-> `UpgradeStrategy` can be either `RollingUpdate` or `OnDelete`
-> `RollingUpdate` uses `MaxUnavailable` field (1 pod will go down for upgrade at a time by default, can be set by user). If user sets MaxUnavailable to 2,
    2 pods will go down for upgrade at once and then the next 2 and so on. This is triggered by CR update shown in Upgrade Steps section

-> `OnDelete`: Upgrade of image will happen for the pod only when user manually deletes the pod. When it comes back up, it comes back with the new image.
    In this case, CR update will not trigger any upgrade without user intervention of deleting each pod.

Note: MaxUnavailable field is meaningful only when UpgradeStrategy is set to “RollingUpdate”. If UpgradeStrategy is set to “OnDelete” and MaxUnavailable is set to an integer, behaviour of OnDelete is still as explained above

## Upgrade Steps

### 1. Verify Cluster Readiness

Ensure the cluster is healthy and CR is already applied and ready for the upgrade. A typical cluster of 2 worker nodes with CR applied will look like this before an upgrade:

```yaml
kube-amd-network  amd-network-operator-multus-multus-tz82k                             1/1     Running   0          32m
kube-amd-network  amd-network-operator-multus-multus-w4v5b                             1/1     Running   0          32m
kube-amd-network  amd-network-operator-network-operator-charts-controller-maf976w      1/1     Running   0          32m
kube-amd-network  amd-network-operator-kmm-controller-6746f8cbc7-lpjxd                 1/1     Running   0          32m
kube-amd-network  amd-network-operator-kmm-webhook-server-6ff4c684bd-bgrs4             1/1     Running   0          32m
kube-amd-network  amd-network-operator-node-feature-discovery-gc-78989c896-m66jp       1/1     Running   0          32m
kube-amd-network  amd-network-operator-node-feature-discovery-master-b8bffc48b-r2p79   1/1     Running   0          32m
kube-amd-network  amd-network-operator-node-feature-discovery-worker-2j2mq             1/1     Running   0          32m
kube-amd-network  amd-network-operator-node-feature-discovery-worker-phb74             1/1     Running   0          32m
kube-amd-network  test-networkconfig-cni-plugins-gsd76                                 1/1     Running   0          17m
kube-amd-network  test-networkconfig-cni-plugins-xwt6h                                 1/1     Running   0          26m
kube-amd-network  test-networkconfig-device-plugin-99k64                               1/1     Running   0          26m
kube-amd-network  test-networkconfig-device-plugin-jsftg                               1/1     Running   0          17m
kube-amd-network  test-networkconfig-metrics-exporter-4mjv2                            1/1     Running   0          26m
kube-amd-network  test-networkconfig-metrics-exporter-9wwc4                            1/1     Running   0          17m
kube-amd-network  test-networkconfig-node-labeller-dxlfr                               1/1     Running   0          26m
kube-amd-network  test-networkconfig-node-labeller-j7xsf                               1/1     Running   0          17m
```

All pods should be in the `Running` state. Resolve any issues such as restarts or errors before proceeding.

### 2. Check Current Image of Device Plugin before Upgrade

The current image the Device Plugin Daemonset is using can be checked by using `kubectl describe <pod-name> -n kube-amd-network` on one of the device plugin pods.

```yaml
device-plugin:
    Container ID:   containerd://b1aaa67ebdd87d4ef0f2a32b76b428068d24c28ced3e86c3c5caba39bb5689a4
    Image:          rocm/k8s-network-device-plugin:v1.1.0
```

### 3. Upgrade the Image of Device Plugin Daemonset

In the Custom Resource, we have the `UpgradePolicy` field in the DevicePluginSpec of type `DaemonSetUpgradeSpec` to support daemonset upgrades. This leverages standard k8s daemonset upgrade support whose details can be found at: https://kubernetes.io/docs/tasks/manage-daemon/update-daemon-set/

To upgrade the device plugin image, we need to update the DevicePluginSpec.DevicePluginImage and set the DevicePluginSpec.UpgradePolicy in the CR.

Example:

Old CR:

```yaml
    devicePlugin:
        devicePluginImage: rocm/k8s-network-device-plugin:v1.1.0
```

Updated CR:

```yaml
    devicePlugin:
        devicePluginImage: rocm/k8s-network-device-plugin:latest
        upgradePolicy:
          upgradeStrategy: RollingUpdate
          maxUnavailable: 1
```

Once the new CR is applied, each device plugin pod will go down 1 at a time and come back with the new image mentioned in the CR.

The new image the Device Plugin Daemonset is using can be checked by using `kubectl describe <pod-name> -n kube-amd-network` on one of the device plugin pods.

```yaml
device-plugin:
    Container ID:   containerd://8b35722a47100f61e9ea4fee4ecf61faa078b7ab36084b2dd0ed8ba00179a883
    Image:          rocm/k8s-network-device-plugin:latest
```

### 4. How to Upgrade Image of NodeLabeller, Metrics Exporter and CNI Plugins Daemonset

-> The upgrade for Nodelabeller works the exact same way as for DevicePlugin. The upgradePolicy mentioned in the DevicePluginSpec applies for both DevicePlugin Daemonset as well as Nodelabeller Daemonset. The only difference is that, in this case, the user will change devicePluginSpec.NodeLabellerImage to trigger the upgrade

-> The upgrade for MetricsExporter needs an UpgradePolicy mentioned in the MetricsExporterSpec. The upgradePolicy has the same 2 fields here as well and the behaviour is the same

Example:

Old CR:

```yaml
  metricsExporter:
    enable: True
    serviceType: "ClusterIP"
    port: 5001
    image: rocm/device-metrics-exporter:nic-v1.1.0
```

Updated CR:

```yaml
  metricsExporter:
    enable: True
    serviceType: "ClusterIP"
    port: 5001
    image: rocm/device-metrics-exporter:nic-v1.1.0
    upgradePolicy:
      upgradeStrategy: OnDelete
```

Once the new CR is applied, each metrics exporter pod has to be brought down manually by user intervention to trigger upgrade for that pod. This is because, in this case, `OnDelete` option is used as upgradeStrategy. The image can be verified the same way as device plugin pod.

-> The upgrade for CNIPlugins requires an upgradePolicy to be specified in the CNIPluginsSpec. The upgradePolicy contains the same two fields as in the other operands mentioned above, and its behavior is consistent with them.

#### **Notes**

- If no UpgradePolicy is mentioned for any of the components but their image is changed in the CR update, the daemonset will get upgraded according to the defaults, which is `UpgradeStrategy` set to `RollingUpdate` and `MaxUnavailable` set to 1.
