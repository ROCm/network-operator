# Uninstall

## Uninstall Helm Charts

```bash
helm uninstall amd-network-operator -n kube-amd-network --debug
```

By default, the helm uninstall command triggers a pre-delete hook that removes all existing `NetworkConfig` resources across all namespaces in the cluster. It also deletes other installed CRDs, such as `NetworkAttachmentDefinition` from `Multus` and `NodeFeatureRule` from `NFD`, if they were created using the flags `--set multus.enabled=true` and `--set node-feature-discovery.enabled=true`.

```{note}
The pre-delete hook is using the operator controller image to run kubectl for checking existing `NetworkConfig`, if you want to skip the pre-delete hook, you can run helm uninstall command with ```--no-hooks``` option, in that way the Helm Charts will be immediately uninstalled but may have risk that some `NetworkConfig` resources still remain in the cluster.
```
