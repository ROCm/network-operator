# Upgrades


### 1. Verify Cluster Readiness

Ensure the cluster is healthy and ready for the upgrade. A typical system will look like this before an upgrade:

```
root@genoa4:~# kubectl get pods -A
NAMESPACE          NAME                                                              READY   STATUS    RESTARTS      AGE
kube-amd-network   amd-network-operator-multus-multus-d6pft                          1/1     Running   14s
kube-amd-network   amd-network-operator-network-operator-charts-controller-mabsz8t   1/1     Running   14s
kube-amd-network   amd-network-operator-node-feature-discovery-gc-77d6d6449c-rcvm8   1/1     Running   14s
kube-amd-network   amd-network-operator-node-feature-discovery-master-869f4bbcczz5   1/1     Running   14s
kube-amd-network   amd-network-operator-node-feature-discovery-worker-pl8xc          1/1     Running   13s
kube-amd-network   nc-cni-plugins-rphjr                                              1/1     Running   124m
kube-amd-network   nc-device-plugin-l6m8z                                            1/1     Running   124m
kube-amd-network   nc-metrics-exporter-mdz2z                                         1/1     Running   124m
kube-flannel       kube-flannel-ds-tl75v                                             1/1     Running   43d
kube-system        coredns-668d6bf9bc-hdh2d                                          1/1     Running   43d
kube-system        coredns-668d6bf9bc-k7jps                                          1/1     Running   43d
kube-system        etcd-genoa4                                                       1/1     Running   43d
kube-system        kube-apiserver-genoa4                                             1/1     Running   43d
kube-system        kube-controller-manager-genoa4                                    1/1     Running   43d
kube-system        kube-proxy-fstqn                                                  1/1     Running   21d
kube-system        kube-scheduler-genoa4                                             1/1     Running   43d
```

All pods should be in the `Running` state. Resolve any issues such as restarts or errors before proceeding.


### 2. Understand Upgrade Safeguards

**Pre-Upgrade Hook**

* ```upgrade-crd```: This hook helps users to patch the new version Custom Resource Definition (CRD) to the helm deployment. Helm by default doesn't support automatic upgrade of CRD so we implemented this hook for auto-upgrade the CRDs.

- **Skipping the Hook:** If necessary, you can bypass the pre-upgrade hook (not recommended) by adding ```--no-hooks```, you would have to manually use new version's CRD to upgrade then in cluster.


### 3. Perform the Upgrade

Upgrade the operator using the following command:

```bash
helm upgrade amd-network-operator network-operator-helm-k8s-v1.0.0.tgz \
    -n kube-amd-network \
    --recreate-pods \
    --debug
```

* When upgrading a Helm chart, customized operator controller image URLs set in the older version's values.yaml (via `--set` or `-f values.yaml`) will persist due to default Helm behavior.
* To ensure a successful upgrade, you must use the target version's operator image in the helm upgrade command. This is because upgrade hooks rely on the target version's images for CRD updates. For example, to upgrade to v1.3.0 when you already customized operator image URL in old version helm chart, use `--set` to ask helm for using correct version image for executing helm upgrade hooks:

```bash
# Perform helm upgrade
helm upgrade amd-network-operator network-operator-helm-k8s-v1.0.0.tgz \
  -n kube-amd-network \
  --version=v1.3.0 \
  --debug \
  --set controllerManager.manager.image.repository=docker.io/rocm/network-operator \
  --set controllerManager.manager.image.tag=v1.3.0 
```

```{note}
Upgrade Options:
* If you encounter the pre-upgrade hook failure and wish to bypass it, please use `--no-hooks` option, then you need to manually patch to upgrade the new version CRDs in the cluster.
* **Error Scenario**: In case there is chart name or release name mismatch happened, you can use `--set fullnameOverride=amd-network-operator-network-operator-charts --set nameOverride=network-operator-charts` to resolve the conflict. The ```fullnameOverride``` and ```nameOverride``` parameters are used to ensure consistent naming between the previous and new chart deployments, avoiding conflicts caused by name mismatches during the upgrade process. The ```fullnameOverride``` explicitly sets the fully qualified name of the resources created by the chart, such as service accounts and deployments. The ```nameOverride``` adjusts the base name of the chart without affecting resource-specific names.
```

### 4. Verify Post-Upgrade State

After the upgrade, ensure all components are running:

```bash
kubectl get pods -n kube-amd-network
```

Verify that nodes are labeled and GPUs are detected:

```bash
kubectl get nodes -oyaml | grep "amd.com/nic"
kubectl get nodes -oyaml | grep "amd.com/vnic"
kubectl get networkconfigs -n kube-amd-network -oyaml
```

#### **Notes**

- Use `--no-hooks` only if necessary and after assessing the potential impact.
- For additional troubleshooting, check operator logs:
  ```bash
  kubectl logs -f -n kube-amd-network amd-network-operator-network-operator-charts-controller-mav4rn9
  ```
