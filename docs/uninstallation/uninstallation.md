# Uninstall

To remove the operator and related resources, you need to follow specific sequence to remove them.

1. `NetworkConfig` custom resources
2. Helm Charts
3. Custom resource definition

## Uninstall Custom Resource

To delete the `Networkconfig`, you can use either one of the methods:

* use existing YAML file: ```kubectl delete -f networkconfig.yaml```
* query the cluster and delete by resource metadata:
  ```kubectl delete networkconfigs <your-networkconfig-name> -n kube-amd-network```
* simply remove all deviceconfigs in the namespace:
  ```kubectl delete networkconfigs --all -n kube-amd-network```

Once the deletion was triggered, if out-of-tree driver was previously installed by AMD Network operator, it will trigger KMM to send worker pods to selected nodes and start to unload the `ionic_rdma` kernel module.

The delete request won't finish immediately, instead it will wait for the unload confirmation from all selected worker nodes, then finish the deletion of the `NetworkConfig` resource.

If delete request gets stuck for too long, you may need to check the status of KMM worker pods, if any error happened please check the worker pod error logs:

```kubectl logs kmm-worker -n kube-amd-network```

or refer to [Troubleshooting](../troubleshooting.md) document to find the solution.

## Uninstall Helm Charts

```bash
helm uninstall amd-network-operator -n kube-amd-network --debug
```

By default, the helm uninstall command triggers a pre-delete hook that removes all existing `NetworkConfig` resources across all namespaces in the cluster. It also deletes other installed CRDs, such as `NetworkAttachmentDefinition` from `Multus` and `NodeFeatureRule` from `NFD`, if they were created using the flags `--set multus.enabled=true` and `--set node-feature-discovery.enabled=true`.

```{note}
The pre-delete hook is using the operator controller image to run kubectl for checking existing `NetworkConfig`, if you want to skip the pre-delete hook, you can run helm uninstall command with ```--no-hooks``` option, in that way the Helm Charts will be immediately uninstalled but may have risk that some `NetworkConfig` resources still remain in the cluster.
```


## Uninstall Custom Resource Definition

By default Helm Charts are using a post-delete hook to uninstall the CRDs for users. If the Helm Charts uninstallation was running with ```--no-hooks``` you may need to manually clean up CRDs after uninstalling the Helm Charts. To list all existing CRDs, run this command:

```bash
$ kubectl get crds
NAME                                             CREATED AT
certificaterequests.cert-manager.io              2025-10-16T21:32:40Z
certificates.cert-manager.io                     2025-10-16T21:32:40Z
challenges.acme.cert-manager.io                  2025-10-16T21:32:40Z
ciliumcidrgroups.cilium.io                       2025-10-16T20:18:15Z
ciliumclusterwidenetworkpolicies.cilium.io       2025-10-16T20:18:14Z
ciliumendpoints.cilium.io                        2025-10-16T20:18:12Z
ciliumidentities.cilium.io                       2025-10-16T20:18:10Z
ciliuml2announcementpolicies.cilium.io           2025-10-16T20:18:17Z
ciliumloadbalancerippools.cilium.io              2025-10-16T20:18:16Z
ciliumnetworkpolicies.cilium.io                  2025-10-16T20:18:13Z
ciliumnodeconfigs.cilium.io                      2025-10-16T20:18:18Z
ciliumnodes.cilium.io                            2025-10-16T20:18:09Z
ciliumpodippools.cilium.io                       2025-10-16T20:18:11Z
clusterissuers.cert-manager.io                   2025-10-16T21:32:40Z
issuers.cert-manager.io                          2025-10-16T21:32:40Z
modules.kmm.sigs.x-k8s.io                        2025-10-22T21:15:10Z
network-attachment-definitions.k8s.cni.cncf.io   2025-10-22T21:15:10Z
networkconfigs.amd.com                           2025-10-22T21:15:10Z
nodefeaturegroups.nfd.k8s-sigs.io                2025-10-22T21:15:10Z
nodefeaturerules.nfd.k8s-sigs.io                 2025-10-22T21:15:10Z
nodefeatures.nfd.k8s-sigs.io                     2025-10-22T21:15:10Z
nodemodulesconfigs.kmm.sigs.x-k8s.io             2025-10-22T21:15:10Z
orders.acme.cert-manager.io                      2025-10-16T21:32:40Z
preflightvalidations.kmm.sigs.x-k8s.io           2025-10-22T21:15:11Z
```

then use kubectl to delete CRDs that need to be deleted:

```bash
kubectl delete crds networkconfigs.amd.com
```

```{warning}
Carefully evaluate the impact of removing all CRDs. If the CRDs of cert-manager, NFD or KMM are being used by operators other than AMD Network operator, deleting those CRDs may affect other operators.
```