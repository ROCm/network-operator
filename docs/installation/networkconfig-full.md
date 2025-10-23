# Custom Resource Guide

NetworkConfig CR with a comprehensive list of configuration options:

```yaml
apiVersion: amd.com/v1alpha1
kind: NetworkConfig
metadata:
  name: test-networkconfig
  # namespace where AMD Network Operator is running
  namespace: kube-amd-network
spec:
  driver:
    enable: true
    # Blacklist amd network drivers on the host. Node reboot is required to apply the blacklist on the worker nodes.
    blacklist: true
    AMDNetworkInstallerRepoURL: "https://repo.radeon.com"
    # DO NOT input the image tag, operator will automatically handle the image tag
    image: "registry.example.com/username/amdainic_kmods"
    # Specify the credential for your private registry if it requires credential to get pull/push access
    # You can create the docker-registry type secret by running command like:
    # kubectl create secret docker-registry my-secret -n kube-amd-network --docker-username=xxx --docker-password=xxx
    # Make sure you created the secret within the namespace that KMM operator is running
    imageRegistrySecret:
      name: my-secret
    imageRegistryTLS:
      insecure: true
      insecureSkipTLSVerify: true
    version: 1.117.1-a-63
    imageSign:
      keySecret:
        name: privateKeySecret
      certSecret:
        name: publicKeySecret
    upgradePolicy:
      # -- enable/disable automatic driver upgrade feature 
      enable: false
      # -- how many nodes can be upgraded in parallel
      maxParallelUpgrades: 5
      # -- maximum number of nodes that can be in a failed upgrade state beyond which upgrades will stop to keep cluster at a minimal healthy state
      maxUnavailableNodes: 50%
      # -- whether reboot each worker node or not during the driver upgrade
      rebootRequired: false
      nodeDrainPolicy:
        # -- whether force draining is allowed or not
        force: false
        # -- the length of time in seconds to wait before giving up drain, zero means infinite
        timeoutSeconds: 600
        # -- the time kubernetes waits for a pod to shut down gracefully after receiving a termination signal, zero means immediate, minus value means follow pod defined grace period
        gracePeriodSeconds: -2
      podDeletionPolicy:
        # -- whether force deletion is allowed or not
        force: false
        # -- the length of time in seconds to wait before giving up on pod deletion, zero means infinite
        timeoutSeconds: 600
        # -- the time kubernetes waits for a pod to shut down gracefully after receiving a termination signal, zero means immediate, minus value means follow pod defined grace period
        gracePeriodSeconds: -2
  # Device plugin and Node labeller config
  devicePlugin:
    devicePluginImage: docker.io/rocm/k8s-network-device-plugin:v1.0.0
    devicePluginImagePullPolicy: "Always"
    devicePluginTolerations:
      - key: "example-key"
        operator: "Equal"
        value: "example-value"
        effect: "NoSchedule"
      - key: "example-key2"
        operator: "Equal"
        value: "example-value2"
        effect: "NoExecute"
    enableNodeLabeller: True
    nodeLabellerImage: docker.io/rocm/k8s-network-node-labeller:v1.0.0
    nodeLabellerImagePullPolicy: "Always"
    nodeLabellerTolerations:
      - key: "example-key"
        operator: "Equal"
        value: "example-value"
        effect: "NoSchedule"
    imageRegistrySecret:
      name: my-secret
    upgradePolicy:
      # the type of daemonset upgrade, RollingUpdate or OnDelete
      upgradeStrategy: OnDelete
      # the maximum number of Pods that can be unavailable during the update process
      maxUnavailable: 5
  # Metrics exporter config
  metricsExporter:
    enable: True
    port: 5001
    serviceType: "NodePort"
    nodePort: 32500
    image: docker.io/rocm/device-metrics-exporter:nic-v1.0.0
    imagePullPolicy: "Always"
    imageRegistrySecret:
      name: my-secret
    upgradePolicy:
      upgradeStrategy: RollingUpdate
      maxUnavailable: 5
    hostNetwork: true
    config:
      name: metricsConfig
    tolerations:
      - key: "example-key"
        operator: "Equal"
        value: "example-value"
        effect: "NoSchedule"
    # selector describes on which nodes to enable metrics exporter
    selector:
      "exporter": "true"
    # kube-rbac-proxy config to provide rbac services
    rbacConfig:
      enable: true
      image: quay.io/brancz/kube-rbac-proxy:latest
      disableHttps: false
      secret:
        name: rbacProxySecret
      clientCAConfigMap:
        name: clientCA
      staticAuthorization:
        enable: true
        clientName: "test"
  # Secondary network config
  secondaryNetwork:
    cniPlugins:
      enable: True
      image: docker.io/rocm/k8s-cni-plugins:v1.0.0
      imagePullPolicy: "Always"
      imageRegistrySecret:
        name: my-secret
      tolerations:
        - key: "example-key"
          operator: "Equal"
          value: "example-value"
          effect: "NoSchedule"
      upgradePolicy:
        upgradeStrategy: RollingUpdate
        maxUnavailable: 5

  commonConfig:
    # -- init container image
    initContainerImage: busybox:1.36
    utilsContainer:
      # -- network operator utility container image used for driver upgrade
      image: docker.io/rocm/network-operator-utils:v1.0.0
      # -- utility container image pull policy
      imagePullPolicy: IfNotPresent
      # -- utility container image pull secret, e.g. {"name": "mySecretName"}
      imageRegistrySecret: {}
  
  # Specify the node to be managed by this NetworkConfig Custom Resource
  selector:
    feature.node.kubernetes.io/amd-nic: "true"
```
