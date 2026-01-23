# Driver Upgrade Guide

This guide walks through the process of upgrading AMD Network drivers on worker nodes.

## Overview

The driver can be upgraded in the following methods.

1. Automatic Upgrade Process
2. Manual Upgrade Process

## 1. Automatic Upgrade Process

Automatic upgrade is enabled when the NetworkConfig has the `upgradePolicy` field set.
If this field is not configured, the user has to follow the manual steps in the next section.
If this field is configured and the `version` field is changed in the driver spec, the automatic driver upgrade process is initiated.

The following operations are sequentially executed by the network operator for each selected node

1. The node is cordoned so that no pods can be scheduled on this node.
2. The existing pods that require AMD NICs, VNICs, or GPUs (i.e., pods requesting `amd.com/nic`, `amd.com/vnic`, or `amd.com/gpu`) are drained/deleted based on the config in the upgrade policy.
3. The desired driver version label is updated as shown below.

   ```bash
   kmm.node.kubernetes.io/version-module.<networkconfig-namespace>.<networkconfig-name>=<new-version>
   ```

4. KMM operator unloads the old driver version and loads the new driver version.
    - The `amdgpu` kernel modules will be reloaded on the node as part of the process. This is primarily due to the `ib_peer_mem` dependency issue between the `amdgpu` and `ionic_rdma` drivers. Since `amdgpu` holds `ib_peer_mem`, the `ionic_rdma` driver cannot be uninstalled while it’s in use. As a result:
        - `amdgpu` is unloaded.
        - `ionic_rdma` is upgraded.
        - `amdgpu` is loaded back.

5. If the node requires reboot post installation (configurable in upgradePolicy), the node is rebooted
6. Once the node is rebooted and the desired driver is loaded, the node is uncordoned and available for scheduling.

The following are the steps to perform the automatic driver upgrade

1. Set the desired driver version and configure upgrade policy
2. Track the upgrade status through CR status

### 1. Set desired driver version and configure upgrade policy

The following sample config shows the relevant fields to start automatic driver upgrade across the nodes in the cluster with default upgrade configuration.

```yaml
apiVersion: amd.com/v1alpha1
kind: NetworkConfig
metadata:
  name: test-networkconfig
  # use the namespace where AMD Network Operator is running
  namespace: kube-amd-network
spec:
  driver:
    version: 1.117.1-a-63
    enable: true
    upgradePolicy:
      enable: true
  selector:
    feature.node.kubernetes.io/amd-vnic: "true"
```

Upgrade configuration reference

To check the full spec of upgrade configuration run kubectl get crds networkconfigs.amd.com -oyaml

#### `driver.upgradePolicy` Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `enable` | Enable this upgrade policy | `false` |
| `maxParallelUpgrades` | Maximum number of nodes which will be upgraded in parallel | `1` |
| `maxUnavailableNodes` | Maximum number (or Percentage) of nodes which can be unavailable (cordoned) in the cluster | `25%` |
| `rebootRequired` | Reboot the node after driver upgrade is done. Waits for 60 mins post reboot before declaring as failed | `true` |

#### `driver.upgradePolicy.nodeDrainPolicy` Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `force` | Allow drain to proceed on the node even if there are managed pods such as daemon-sets. In such cases drain will not proceed unless this option is set to true | `true` |
| `timeout` | The length of time to wait before giving up. Zero means infinite | `300s` |

#### `driver.upgradePolicy.podDeletionPolicy` Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `force` | Force delete all pods that use amd nics | `true` |
| `timeout` | The length of time to wait before giving up. Zero means infinite | `300s` |

### 2. Track the upgrade status through CR status

The `status.nodeModuleStatus.<worker-node>.status` captures the status of the upgrade process for each node

```yaml
status:
  nodeModuleStatus:
    cloudvm1:
      bootId: fdd8ac0e-777b-4e0e-88d4-3d92300d8b65
      containerImage: docker.io/amdpsdo/nic-driver:ubuntu-24.04-6.8.0-85-generic-1.117.1-a-63
      kernelVersion: 6.8.0-85-generic
      lastTransitionTime: 2025-10-16 06:02:48 +0000 UTC
      status: Upgrade-Complete
      upgradeStartTime: 2025-10-16 06:02:26 UTC
    cloudvm2:
      bootId: 5b40ad07-e3ff-48ef-9c78-718522ffcb74
      containerImage: docker.io/amdpsdo/nic-driver:ubuntu-24.04-6.8.0-85-generic-1.117.1-a-63
      kernelVersion: 6.8.0-85-generic
      lastTransitionTime: 2025-10-16 06:03:52 +0000 UTC
      status: Upgrade-Complete
      upgradeStartTime: 2025-10-16 06:03:30 UTC
    cloudvm3:
      bootId: 786010b3-e81c-4c0c-bd2d-5d6ed986ed7e
      containerImage: docker.io/amdpsdo/nic-driver:ubuntu-24.04-6.8.0-85-generic-1.117.1-a-63
      kernelVersion: 6.8.0-85-generic
      lastTransitionTime: 2025-10-16 06:04:02 +0000 UTC
      status: Upgrade-Complete
      upgradeStartTime: 2025-10-16 06:03:56 UTC
    cloudvm4:
      bootId: a4a8a4f6-c012-4087-baca-51e6638c4691
      containerImage: docker.io/amdpsdo/nic-driver:ubuntu-24.04-6.8.0-85-generic-1.117.1-a-63
      kernelVersion: 6.8.0-85-generic
      lastTransitionTime: 2025-10-16 06:04:00 +0000 UTC
      status: Upgrade-Complete
      upgradeStartTime: 2025-10-16 01:59:11 UTC
```

The following are the different node states during the upgrade process

| State | Description |
|-----------|---------|
| `Install-In-Progress` | Driver is being installed on the node for the first time |
| `Install-Complete` | Driver install is complete |
| `Upgrade-Not-Started` | Automatic upgrade enabled and driver version change is detected. All nodes move to this state |
| `Upgrade-In-Progress` | Selected nodes conforming to upgrade policy will be attempted for driver upgrade |
| `Upgrade-Complete` | Driver upgrade is successfully complete on the node |
| `Upgrade-Timed-Out` | Driver upgrade couldn't finish within 2 hours |
| `Cordon-Failed` | Cordoning of the node failed |
| `Uncordon-Failed` | Uncordoning of the node failed |
| `Drain-Failed` | Drain node or Delete pods operation failed|
| `Reboot-In-Progress` | Driver upgrade is done and reboot is in progress |
| `Reboot-Failed` | Driver upgrade is done and reboot attempt failed |
| `Upgrade-Failed` | Driver upgrade failed for any other reasons |

The following are considered during the automatic upgrade process

1. Selection of a node should satisfy both `maxUnavailableNodes` and `maxParallelUpgrades` criteria
2. All nodes in failed state is considered while calculating `maxUnavailableNodes`

### 3. Recovery From Upgrade Failure

If it is observed that the upgrade status is in failed state for a specific node, the user can debug the node, fix it and then add this label to the node to restart upgrade on it. The upgrade state will be reset and it can be tracked as it was before

- Command:   `kubectl label node <nodename> operator.amd.com/network-driver-upgrade-state=upgrade-required`
- Label:     `operator.amd.com/network-driver-upgrade-state: upgrade-required`

## 2. Manual Upgrade Process

The manual upgrade process involves the following steps:

1. Verifying current installation
2. Updating the driver version
3. Managing workloads
4. Updating node labels
5. Performing the upgrade

### 1. Check Current Driver Version

Verify the existing driver version label on your worker nodes:

```bash
kubectl get node <worker-node> -o yaml
```

Look for the label in this format:

```text
kmm.node.kubernetes.io/version-module.<networkconfig-namespace>.<networkconfig-name>=<version>
```

Example:

```text
kmm.node.kubernetes.io/version-module.kube-amd-network.vf-test-networkconfig=1.117.1-a-63
```

### 2. Update NetworkConfig

Update the `driverVersion` field in your NetworkConfig:

```bash
kubectl edit networkconfigs <networkconfig-name> -n kube-amd-network
```

The operator will automatically:

1. Look for the new driver image in the registry
2. Build the image if it doesn't exist
3. Push the built image to your specified registry

#### Image Tag Format

The operator uses specific tag formats based on the OS:

| OS | Tag Format | Example |
|----|------------|---------|
| Ubuntu | `ubuntu-<version>-<kernel>-<driver>` | `ubuntu-22.04-6.8.0-40-generic-6.1.3` |
| RHEL CoreOS | `coreos-<version>-<kernel>-<driver>` | `coreos-416.94-5.14.0-427.28.1.el9_4.x86_64-6.2.2` |

> **Warning**: If a node's ready status changes during upgrade (Ready → NotReady → Ready) before its driver version label is updated, the old driver won't be reinstalled. Complete the upgrade steps for these nodes to install the new driver.

### 3. Stop Workloads

Stop all workloads using the AMD NIC driver on the target node before proceeding.

### 4. Update Node Labels

You have two options for updating node labels:

#### Option A: Direct Update (Recommended)

If no additional maintenance is needed, directly update the version label:

```bash
# Old label format:
kmm.node.kubernetes.io/version-module.<namespace>.<networkconfig-name>=<old-version>
# New label format:
kmm.node.kubernetes.io/version-module.<namespace>.<networkconfig-name>=<new-version>
```

#### Option B: Remove and Add (If maintenance is needed)

- Remove old version label:

```bash
kubectl label node <worker-node> \
  kmm.node.kubernetes.io/version-module.<namespace>.<networkconfig-name>-
```

- Perform required maintenance

- Add new version label:

```bash
kubectl label node <worker-node> \
 kmm.node.kubernetes.io/version-module.<namespace>.<networkconfig-name>=<new-version>
```

### 5. Restart Workloads

After the new driver is installed successfully, restart your NIC workloads on the upgraded node.

## Verification

To verify the upgrade, check node labels:

```bash
kubectl get node <worker-node> --show-labels | grep kmm.node.kubernetes.io
```

- Verify driver version:

```bash
kubectl get networkconfigs <networkconfig-name> -n kube-amd-network -o yaml
```

- Check driver status:

```bash
kubectl get networkconfigs <networkconfig-name> -n kube-amd-network -o jsonpath='{.status}'
```
