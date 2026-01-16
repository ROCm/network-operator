/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
Copyright (c) Advanced Micro Devices, Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the \"License\");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an \"AS IS\" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type DevicePluginSpec struct {
	// device plugin image
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="DevicePluginImage",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:devicePluginImage"}
	// +optional
	// +kubebuilder:validation:Pattern=`^([a-z0-9]+(?:[._-][a-z0-9]+)*(:[0-9]+)?)(/[a-z0-9]+(?:[._-][a-z0-9]+)*)*(?::[a-z0-9._-]+)?(?:@[a-zA-Z0-9]+:[a-f0-9]+)?$`
	DevicePluginImage string `json:"devicePluginImage,omitempty"`

	// image pull policy for device plugin
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="DevicePluginImagePullPolicy",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:DevicePluginImagePullPolicy"}
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	// +optional
	DevicePluginImagePullPolicy string `json:"devicePluginImagePullPolicy,omitempty"`

	// tolerations for the device plugin DaemonSet
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="DevicePluginTolerations",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:devicePluginTolerations"}
	// +optional
	DevicePluginTolerations []v1.Toleration `json:"devicePluginTolerations,omitempty"`

	// device plugin arguments is used to pass supported flags and their values while starting device plugin daemonset
	// supported flag values: {"resource_naming_strategy": {"single", "mixed"}}
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="DevicePluginArguments",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:devicePluginArguments"}
	// +optional
	DevicePluginArguments map[string]string `json:"devicePluginArguments,omitempty"`

	// node labeller image
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="NodeLabellerImage",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:nodeLabellerImage"}
	// +optional
	// +kubebuilder:validation:Pattern=`^([a-z0-9]+(?:[._-][a-z0-9]+)*(:[0-9]+)?)(/[a-z0-9]+(?:[._-][a-z0-9]+)*)*(?::[a-z0-9._-]+)?(?:@[a-zA-Z0-9]+:[a-f0-9]+)?$`
	NodeLabellerImage string `json:"nodeLabellerImage,omitempty"`

	// image pull policy for node labeller
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="NodeLabellerImagePullPolicy",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:NodeLabellerImagePullPolicy"}
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	// +optional
	NodeLabellerImagePullPolicy string `json:"nodeLabellerImagePullPolicy,omitempty"`

	// tolerations for the node labeller DaemonSet
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="NodeLabellerTolerations",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:nodeLabellerTolerations"}
	// +optional
	NodeLabellerTolerations []v1.Toleration `json:"nodeLabellerTolerations,omitempty"`

	// node labeller image registry secret used to pull/push images
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ImageRegistrySecret",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:imageRegistrySecret"}
	// +optional
	ImageRegistrySecret *v1.LocalObjectReference `json:"imageRegistrySecret,omitempty"`

	// enable or disable the node labeller
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="EnableNodeLabeller",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:enableNodeLabeller"}
	// +kubebuilder:default=true
	EnableNodeLabeller *bool `json:"enableNodeLabeller,omitempty"`

	// upgrade policy for device plugin and node labeller daemons
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="UpgradePolicy",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:upgradePolicy"}
	// +optional
	UpgradePolicy *DaemonSetUpgradeSpec `json:"upgradePolicy,omitempty"`
}

type DaemonSetUpgradeSpec struct {
	// UpgradeStrategy specifies the type of the DaemonSet update. Valid values are "RollingUpdate" (default) or "OnDelete".
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="UpgradeStrategy",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:upgradeStrategy"}
	// +kubebuilder:validation:Enum=RollingUpdate;OnDelete
	// +optional
	UpgradeStrategy string `json:"upgradeStrategy,omitempty"`

	// MaxUnavailable specifies the maximum number of Pods that can be unavailable during the update process. Applicable for RollingUpdate only. Default value is 1.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="MaxUnavailable",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:maxUnavailable"}
	// +kubebuilder:default=1
	MaxUnavailable int32 `json:"maxUnavailable,omitempty"`
}

// NetworkConfigSpec describes how the AMD Network operator should enable AMD Network device for customer's use.
type NetworkConfigSpec struct {
	// driver
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Driver",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:driver"}
	// +optional
	Driver DriverSpec `json:"driver,omitempty"`

	// metrics exporter
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="MetricsExporter",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:metricsExporter"}
	// +optional
	MetricsExporter MetricsExporterSpec `json:"metricsExporter,omitempty"`

	// config manager
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ConfigManager",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:configManager"}
	// +optional
	ConfigManager ConfigManagerSpec `json:"configManager,omitempty"`

	// device plugin
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="DevicePlugin",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:devicePlugin"}
	// +optional
	DevicePlugin DevicePluginSpec `json:"devicePlugin,omitempty"`

	// test runner
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="TestRunner",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:testRunner"}
	// +optional
	TestRunner TestRunnerSpec `json:"testRunner,omitempty"`

	// common config
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="CommonConfig",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:commonConfig"}
	// +optional
	CommonConfig CommonConfigSpec `json:"commonConfig,omitempty"`

	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="SecondaryNetwork",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:secondaryNetwork"}
	// +optional
	// SecondaryNetworkSpec contains the spec for secondary network: CNI plugins and IPAM
	SecondaryNetwork SecondaryNetworkSpec `json:"secondaryNetwork,omitempty"`

	// Selector describes on which nodes the Network Operator should enable the Network device.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Selector",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:selector"}
	// +optional
	Selector map[string]string `json:"selector,omitempty"`
}

type CniPluginsSpec struct {
	// enable CNI plugins, disabled by default
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Enable",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:enable"}
	// +optional
	Enable *bool `json:"enable,omitempty"`

	// image information for CNI plugins
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Image",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:image"}
	// +optional
	// +kubebuilder:validation:Pattern=`^([a-z0-9]+(?:[._-][a-z0-9]+)*(:[0-9]+)?)(/[a-z0-9]+(?:[._-][a-z0-9]+)*)*(?::[a-z0-9._-]+)?(?:@[a-zA-Z0-9]+:[a-f0-9]+)?$`
	Image string `json:"image,omitempty"`

	// image registry secret used to pull/push images
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ImageRegistrySecret",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:imageRegistrySecret"}
	// +optional
	ImageRegistrySecret *v1.LocalObjectReference `json:"imageRegistrySecret,omitempty"`

	// image pull policy
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ImagePullPolicy",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:imagePullPolicy"}
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	// +optional
	ImagePullPolicy string `json:"imagePullPolicy,omitempty"`

	// tolerations
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Tolerations",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:tolerations"}
	// +optional
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`

	// upgrade policy for CNI plugins daemonset
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="UpgradePolicy",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:upgradePolicy"}
	// +optional
	UpgradePolicy *DaemonSetUpgradeSpec `json:"upgradePolicy,omitempty"`
}

type SecondaryNetworkSpec struct {
	// Image information for CNI plugins
	CniPlugins *CniPluginsSpec `json:"cniPlugins,omitempty"`
}

type RegistryTLS struct {
	// If true, check if the container image already exists using plain HTTP.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Insecure",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:insecure"}
	// +optional
	Insecure *bool `json:"insecure,omitempty"`
	// If true, skip any TLS server certificate validation
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="InsecureSkipTLSVerify",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:insecureSkipTLSVerify"}
	// +optional
	InsecureSkipTLSVerify *bool `json:"insecureSkipTLSVerify,omitempty"`
}

type DriverSpec struct {
	// enable driver install. default value is true.
	// disable is for skipping driver install/uninstall for dryrun or using in-tree ionic and rdma related kernel modules
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Enable",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:enable"}
	// +kubebuilder:default=true
	Enable *bool `json:"enable,omitempty"`

	// blacklist amdnetwork drivers on the host. Node reboot is required to apply the blacklist on the worker nodes.
	// Require spec.devicePlugin.enableNodeLabeller=true to take effect.
	// Not working for OpenShift cluster. OpenShift users please use the Machine Config Operator (MCO) resource to configure ionic blacklist.
	// Example MCO resource is available at https://instinct.docs.amd.com/projects/network-operator/en/latest/installation/openshift-olm.html#create-blacklist-for-installing-out-of-tree-kernel-module
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="BlacklistDrivers",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:blacklistDrivers"}
	Blacklist *bool `json:"blacklist,omitempty"`

	// radeon repo URL for fetching amdnetwork installer if building driver image on the fly
	// installer URL is https://repo.radeon.com by default
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="AMDNetworkInstallerRepoURL",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:AMDNetworkInstallerRepoURL"}
	// +optional
	AMDNetworkInstallerRepoURL string `json:"AMDNetworkInstallerRepoURL,omitempty"`

	// version of the drivers source code, can be used as part of image of dockerfile source image
	// default value for different OS is: ubuntu: 1.117.1-a-42, coreOS: 1.117.1-a-42
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Version",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:version"}
	// +optional
	Version string `json:"version,omitempty"`

	// defines image that includes drivers and firmware blobs, don't include tag since it will be fully managed by operator
	// for vanilla k8s the default value is image-registry:5000/$MOD_NAMESPACE/amdnetwork_kmod
	// for OpenShift the default value is image-registry.openshift-image-registry.svc:5000/$MOD_NAMESPACE/amdnetwork_kmod
	// image tag will be in the format of <linux distro>-<release version>-<kernel version>-<driver version>
	// example tag is coreos-416.94-5.14.0-427.28.1.el9_4.x86_64-6.2.2 and ubuntu-22.04-5.15.0-94-generic-6.1.3
	// NOTE: Updating the driver image repository is not supported. Please delete the existing NetworkConfig and create a new one with the updated image repository
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Image",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:image"}
	// +optional
	// +kubebuilder:validation:Pattern=`^([a-z0-9]+(?:[._-][a-z0-9]+)*(:[0-9]+)?)(/[$a-zA-Z0-9_]+(?:[._-][$a-zA-Z0-9_]+)*)*(?::[a-z0-9._-]+)?(?:@[a-zA-Z0-9]+:[a-f0-9]+)?$`
	Image string `json:"image,omitempty"`

	// driver image registry TLS setting for the container image
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ImageRegistryTLS",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:imageRegistryTLS"}
	// +optional
	ImageRegistryTLS RegistryTLS `json:"imageRegistryTLS,omitempty"`

	// secrets used for pull/push images from/to private registry specified in driversImage
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ImageRegistrySecret",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:imageRegistrySecret"}
	// +optional
	ImageRegistrySecret *v1.LocalObjectReference `json:"imageRegistrySecret,omitempty"`

	// image signing config to sign the driver image when building driver image on the fly
	// image signing is required for installing driver on secure boot enabled system
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ImageSign",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:imageSign"}
	// +optional
	ImageSign ImageSignSpec `json:"imageSign,omitempty"`

	// policy to upgrade the drivers
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="UpgradePolicy",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:upgradePolicy"}
	// +optional
	UpgradePolicy *DriverUpgradePolicySpec `json:"upgradePolicy,omitempty"`

	// tolerations for kmm module object
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Tolerations",xDescriptors={"urn:alm:descriptor:com.amd.deviceconfigs:tolerations"}
	// +optional
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`
}

// UpgradeState captures the state of the upgrade process on a node
// +enum
type UpgradeState string

const (
	// No State.
	UpgradeStateEmpty UpgradeState = ""
	// Node upgrade pending
	UpgradeStateNotStarted UpgradeState = "Upgrade-Not-Started"
	// Node pre-upgrade ops
	UpgradeStateStarted UpgradeState = "Upgrade-Started"
	// Node install in progress
	UpgradeStateInstallInProgress UpgradeState = "Install-In-Progress"
	// Node install complete
	UpgradeStateInstallComplete UpgradeState = "Install-Complete"
	// Node upgrade in progress
	UpgradeStateInProgress UpgradeState = "Upgrade-In-Progress"
	// Node upgrade complete
	UpgradeStateComplete UpgradeState = "Upgrade-Complete"
	// Node upgrade failed
	UpgradeStateFailed UpgradeState = "Upgrade-Failed"
	// Node upgrade timed out
	UpgradeStateTimedOut UpgradeState = "Upgrade-Timed-Out"
	// Node cordon failed
	UpgradeStateCordonFailed UpgradeState = "Cordon-Failed"
	// Node uncordon failed
	UpgradeStateUncordonFailed UpgradeState = "Uncordon-Failed"
	// Node drain failed
	UpgradeStateDrainFailed UpgradeState = "Drain-Failed"
	// Node reboot in progress
	UpgradeStateRebootInProgress UpgradeState = "Reboot-In-Progress"
	// Node reboot failed
	UpgradeStateRebootFailed UpgradeState = "Reboot-Failed"
)

type DriverUpgradePolicySpec struct {
	// enable upgrade policy, disabled by default
	// If disabled, user has to manually upgrade all the nodes.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Enable",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:enable"}
	// +optional
	Enable *bool `json:"enable,omitempty"`
	// MaxParallelUpgrades indicates how many nodes can be upgraded in parallel
	// 0 means no limit, all nodes will be upgraded in parallel
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="MaxParallelUpgrades",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:maxParallelUpgrades"}
	// +optional
	// +kubebuilder:default:=1
	// +kubebuilder:validation:Minimum:=0
	MaxParallelUpgrades int `json:"maxParallelUpgrades,omitempty"`
	// MaxUnavailableNodes indicates maximum number of nodes that can be in a failed upgrade state beyond which upgrades will stop to keep cluster at a minimal healthy state
	// Value can be an integer (ex: 2) which would mean atmost 2 nodes can be in failed state after which new upgrades will not start. Or it can be a percentage string(ex: "50%") from which absolute number will be calculated and round up
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="MaxUnavailableNodes",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:maxUnavailableNodes"}
	// +optional
	// +kubebuilder:default:="25%"
	MaxUnavailableNodes intstr.IntOrString `json:"maxUnavailableNodes,omitempty"`
	// Node draining policy
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="NodeDrainPolicy",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:nodeDrainPolicy"}
	// +optional
	NodeDrainPolicy *DrainSpec `json:"nodeDrainPolicy,omitempty"`
	// Pod Deletion policy. If both NodeDrainPolicy and PodDeletionPolicy config is available, NodeDrainPolicy(if enabled) will take precedence.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="PodDeletionPolicy",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:podDeletionPolicy"}
	// +optional
	PodDeletionPolicy *PodDeletionSpec `json:"podDeletionPolicy,omitempty"`
	// reboot between driver upgrades, enabled by default, if enabled spec.commonConfig.utilsContainer will be used to perform reboot on worker nodes
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="RebootRequired",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:rebootRequired"}
	// +optional
	// +kubebuilder:default:=true
	RebootRequired *bool `json:"rebootRequired,omitempty"`
}

type DrainSpec struct {
	// Force indicates if force draining is allowed
	// +optional
	// +kubebuilder:default:=false
	Force *bool `json:"force,omitempty"`
	// TimeoutSecond specifies the length of time in seconds to wait before giving up drain, zero means infinite
	// +optional
	// +kubebuilder:default:=300
	// +kubebuilder:validation:Minimum:=0
	TimeoutSeconds int `json:"timeoutSeconds,omitempty"`
	// GracePeriodSeconds indicates the time kubernetes waits for a pod to shut down gracefully after receiving a termination signal
	// +optional
	// +kubebuilder:default:=-1
	GracePeriodSeconds int `json:"gracePeriodSeconds,omitempty"`
}

type PodDeletionSpec struct {
	// Force indicates if force deletion is allowed
	// +optional
	// +kubebuilder:default:=false
	Force *bool `json:"force,omitempty"`
	// TimeoutSecond specifies the length of time in seconds to wait before giving up on pod deletion, zero means infinite
	// +optional
	// +kubebuilder:default:=300
	// +kubebuilder:validation:Minimum:=0
	TimeoutSeconds int `json:"timeoutSeconds,omitempty"`
	// GracePeriodSeconds indicates the time kubernetes waits for a pod to shut down gracefully after receiving a termination signal
	// +optional
	// +kubebuilder:default:=-1
	GracePeriodSeconds int `json:"gracePeriodSeconds,omitempty"`
}

type ImageSignSpec struct {
	// ImageSignKeySecret the private key used to sign kernel modules within image
	// necessary for secure boot enabled system
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ImageSignKeySecret",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:imageSignKeySecret"}
	// +optional
	KeySecret *v1.LocalObjectReference `json:"keySecret,omitempty"`

	// ImageSignCertSecret the public key used to sign kernel modules within image
	// necessary for secure boot enabled system
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ImageSignCertSecret",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:imageSignCertSecret"}
	// +optional
	CertSecret *v1.LocalObjectReference `json:"certSecret,omitempty"`
}

// ServiceType string describes ingress methods for a service
type ServiceType string

const (
	// ServiceTypeClusterIP to access inside the cluster
	ServiceTypeClusterIP ServiceType = "ClusterIP"

	// ServiceTypeNodePort to expose service to external
	ServiceTypeNodePort ServiceType = "NodePort"
)

type ConfigManagerSpec struct {
	// enable config manager, disabled by default
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Enable",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:enable"}
	// +optional
	Enable *bool `json:"enable,omitempty"`

	// config manager image
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Image",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:image"}
	// +optional
	// +kubebuilder:validation:Pattern=`^([a-z0-9]+(?:[._-][a-z0-9]+)*(:[0-9]+)?)(/[a-z0-9]+(?:[._-][a-z0-9]+)*)*(?::[a-z0-9._-]+)?(?:@[a-zA-Z0-9]+:[a-f0-9]+)?$`
	Image string `json:"image,omitempty"`

	// image pull policy for config manager
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ImagePullPolicy",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:imagePullPolicy"}
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	// +optional
	ImagePullPolicy string `json:"imagePullPolicy,omitempty"`

	// config manager image registry secret used to pull/push images
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ImageRegistrySecret",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:imageRegistrySecret"}
	// +optional
	ImageRegistrySecret *v1.LocalObjectReference `json:"imageRegistrySecret,omitempty"`

	// config map to customize the config for config manager, if not specified default config will be applied
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Config",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:configmap"}
	// +optional
	Config *v1.LocalObjectReference `json:"config,omitempty"`

	// Selector describes on which nodes to enable config manager
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Selector",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:selector"}
	// +optional
	Selector map[string]string `json:"selector,omitempty"`

	// upgrade policy for config manager daemonset
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="UpgradePolicy",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:upgradePolicy"}
	// +optional
	UpgradePolicy *DaemonSetUpgradeSpec `json:"upgradePolicy,omitempty"`

	// tolerations for the network config manager DaemonSet
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ConfigManagerTolerations",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:configManagerTolerations"}
	// +optional
	ConfigManagerTolerations []v1.Toleration `json:"configManagerTolerations,omitempty"`
}

type MetricsExporterSpec struct {
	// enable metrics exporter, disabled by default
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Enable",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:enable"}
	// +optional
	Enable *bool `json:"enable,omitempty"`

	// metrics exporter image
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Image",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:image"}
	// +optional
	// +kubebuilder:validation:Pattern=`^([a-z0-9]+(?:[._-][a-z0-9]+)*(:[0-9]+)?)(/[a-z0-9]+(?:[._-][a-z0-9]+)*)*(?::[a-z0-9._-]+)?(?:@[a-zA-Z0-9]+:[a-f0-9]+)?$`
	Image string `json:"image,omitempty"`

	// metrics exporter image registry secret used to pull/push images
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ImageRegistrySecret",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:imageRegistrySecret"}
	// +optional
	ImageRegistrySecret *v1.LocalObjectReference `json:"imageRegistrySecret,omitempty"`

	// image pull policy for metrics exporter
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ImagePullPolicy",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:imagePullPolicy"}
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	// +optional
	ImagePullPolicy string `json:"imagePullPolicy,omitempty"`

	// tolerations for metrics exporter
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Tolerations",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:tolerations"}
	// +optional
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`

	// Port is the internal port used for in-cluster and node access to pull metrics from the metrics-exporter (default 5001).
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Port",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:port"}
	// +kubebuilder:default=5001
	Port int32 `json:"port,omitempty"`

	// ServiceType service type for metrics, clusterIP/NodePort, clusterIP by default
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ServiceType",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:serviceType"}
	// +kubebuilder:validation:Enum=ClusterIP;NodePort
	// +kubebuilder:default=ClusterIP
	SvcType ServiceType `json:"serviceType,omitempty"`

	// NodePort is the external port for pulling metrics from outside the cluster, in the range 30000-32767 (assigned automatically by default)
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="NodePort",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:nodePort"}
	// +optional
	// +kubebuilder:validation:Minimum=30000
	// +kubebuilder:validation:Maximum=32767
	NodePort int32 `json:"nodePort,omitempty"`

	// optional configuration for metrics
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Config",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:config"}
	// +optional
	Config MetricsConfig `json:"config,omitempty"`

	// optional kube-rbac-proxy config to provide rbac services
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="RbacConfig",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:rbacConfig"}
	// +optional
	RbacConfig KubeRbacConfig `json:"rbacConfig,omitempty"`

	// Selector describes on which nodes to enable metrics exporter
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Selector",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:selector"}
	// +optional
	Selector map[string]string `json:"selector,omitempty"`

	// upgrade policy for metrics exporter daemons
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="UpgradePolicy",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:upgradePolicy"}
	// +optional
	UpgradePolicy *DaemonSetUpgradeSpec `json:"upgradePolicy,omitempty"`

	// HostNetwork enables metrics exporter to use host networking, enabled by default
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="HostNetwork",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:hostNetwork"}
	// +kubebuilder:default=true
	// +optional
	HostNetwork *bool `json:"hostNetwork,omitempty"`

	// Prometheus configuration for metrics exporter
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Prometheus",xDescriptors={"urn:alm:descriptor:com.amd.deviceconfigs:prometheus"}
	// +optional
	Prometheus *PrometheusConfig `json:"prometheus,omitempty"`
}

type PrometheusConfig struct {
	// ServiceMonitor configuration for Prometheus integration
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ServiceMonitor",xDescriptors={"urn:alm:descriptor:com.amd.deviceconfigs:serviceMonitor"}
	// +optional
	ServiceMonitor *ServiceMonitorConfig `json:"serviceMonitor,omitempty"`
}

// ServiceMonitorConfig provides configuration for ServiceMonitor
type ServiceMonitorConfig struct {
	// Enable or disable ServiceMonitor creation (default false)
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Enable",xDescriptors={"urn:alm:descriptor:com.amd.deviceconfigs:enable"}
	// +optional
	Enable *bool `json:"enable,omitempty"`

	// How frequently to scrape metrics. Accepts values with time unit suffix: "30s", "1m", "2h", "500ms"
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Interval",xDescriptors={"urn:alm:descriptor:com.amd.deviceconfigs:interval"}
	// +optional
	// +kubebuilder:validation:Pattern=`^([0-9]+)(ms|s|m|h)$`
	Interval string `json:"interval,omitempty"`

	// AttachMetadata defines if Prometheus should attach node metadata to the target
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="AttachMetadata",xDescriptors={"urn:alm:descriptor:com.amd.deviceconfigs:attachMetadata"}
	// +optional
	AttachMetadata *monitoringv1.AttachMetadata `json:"attachMetadata,omitempty"`

	// HonorLabels chooses the metric's labels on collisions with target labels (default true)
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="HonorLabels",xDescriptors={"urn:alm:descriptor:com.amd.deviceconfigs:honorLabels"}
	// +optional
	// +kubebuilder:default=true
	HonorLabels *bool `json:"honorLabels,omitempty"`

	// HonorTimestamps controls whether the scrape endpoints honor timestamps (default false)
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="HonorTimestamps",xDescriptors={"urn:alm:descriptor:com.amd.deviceconfigs:honorTimestamps"}
	// +optional
	HonorTimestamps *bool `json:"honorTimestamps,omitempty"`

	// Additional labels to add to the ServiceMonitor (default release: prometheus)
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Labels",xDescriptors={"urn:alm:descriptor:com.amd.deviceconfigs:labels"}
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// RelabelConfigs to apply to samples before ingestion
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Relabelings",xDescriptors={"urn:alm:descriptor:com.amd.deviceconfigs:relabelings"}
	// +optional
	Relabelings []monitoringv1.RelabelConfig `json:"relabelings,omitempty"`

	// Relabeling rules applied to individual scraped metrics
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="MetricRelabelings",xDescriptors={"urn:alm:descriptor:com.amd.deviceconfigs:metricRelabelings"}
	// +optional
	MetricRelabelings []monitoringv1.RelabelConfig `json:"metricRelabelings,omitempty"`

	// Optional Prometheus authorization configuration for accessing the endpoint
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization",xDescriptors={"urn:alm:descriptor:com.amd.deviceconfigs:authorization"}
	// +optional
	Authorization *monitoringv1.SafeAuthorization `json:"authorization,omitempty"`

	// Path to bearer token file to be used by Prometheus (e.g., service account token path)
	// Deprecated: Use Authorization instead. This field is kept for backward compatibility.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="BearerTokenFile",xDescriptors={"urn:alm:descriptor:com.amd.deviceconfigs:bearerTokenFile"}
	// +optional
	BearerTokenFile string `json:"bearerTokenFile,omitempty"`

	// TLS settings used by Prometheus to connect to the metrics endpoint
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="TLSConfig",xDescriptors={"urn:alm:descriptor:com.amd.deviceconfigs:tlsConfig"}
	// +optional
	TLSConfig *monitoringv1.TLSConfig `json:"tlsConfig,omitempty"`
}

// StaticAuthConfig contains static authorization configuration for kube-rbac-proxy
type StaticAuthConfig struct {
	// Enables static authorization using client certificate CN
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Enable",xDescriptors={"urn:alm:descriptor:com.amd.deviceconfigs:enable"}
	Enable bool `json:"enable,omitempty"`

	// Expected CN (Common Name) from client cert (e.g., Prometheus SA identity)
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ClientName",xDescriptors={"urn:alm:descriptor:com.amd.deviceconfigs:clientName"}
	ClientName string `json:"clientName,omitempty"`
}

// KubeRbacConfig contains configs for kube-rbac-proxy sidecar
type KubeRbacConfig struct {
	// enable kube-rbac-proxy, disabled by default
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Enable",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:enable"}
	// +optional
	Enable *bool `json:"enable,omitempty"`

	// kube-rbac-proxy image
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Image",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:image"}
	// +optional
	// +kubebuilder:validation:Pattern=`^([a-z0-9]+(?:[._-][a-z0-9]+)*(:[0-9]+)?)(/[a-z0-9]+(?:[._-][a-z0-9]+)*)*(?::[a-z0-9._-]+)?(?:@[a-zA-Z0-9]+:[a-f0-9]+)?$`
	Image string `json:"image,omitempty"`

	// disable https protecting the proxy endpoint
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="DisableHttps",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:disableHttps"}
	// +optional
	DisableHttps *bool `json:"disableHttps,omitempty"`

	// certificate secret to mount in kube-rbac container for TLS, self signed certificates will be generated by default
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Secret",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:secret"}
	// +optional
	Secret *v1.LocalObjectReference `json:"secret,omitempty"`

	// Reference to a configmap containing the client CA (key: ca.crt) for mTLS client validation
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ClientCAConfigMap",xDescriptors={"urn:alm:descriptor:com.amd.deviceconfigs:clientCAConfigMap"}
	// +optional
	ClientCAConfigMap *v1.LocalObjectReference `json:"clientCAConfigMap,omitempty"`

	// Optional static RBAC rules based on client certificate Common Name (CN)
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="StaticAuthorization",xDescriptors={"urn:alm:descriptor:com.amd.deviceconfigs:staticAuthorization"}
	// +optional
	StaticAuthorization *StaticAuthConfig `json:"staticAuthorization,omitempty"`
}

// MetricsConfig contains list of metrics to collect/report
type MetricsConfig struct {
	// Name of the configMap that defines the list of metrics
	// default list:[]
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Name",xDescriptors={"urn:alm:descriptor:com.amd.networkconfigs:name"}
	// +optional
	Name string `json:"name,omitempty"`
}

type TestRunnerSpec struct {
	// enable test runner, disabled by default
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Enable",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:enable"}
	// +optional
	Enable *bool `json:"enable,omitempty"`

	// test runner image
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Image",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:image"}
	// +optional
	// +kubebuilder:validation:Pattern=`^([a-z0-9]+(?:[._-][a-z0-9]+)*(:[0-9]+)?)(/[a-z0-9]+(?:[._-][a-z0-9]+)*)*(?::[a-z0-9._-]+)?(?:@[a-zA-Z0-9]+:[a-f0-9]+)?$`
	Image string `json:"image,omitempty"`

	// image pull policy for test runner
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ImagePullPolicy",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:imagePullPolicy"}
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	// +optional
	ImagePullPolicy string `json:"imagePullPolicy,omitempty"`

	// tolerations for test runner
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Tolerations",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:tolerations"}
	// +optional
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`

	// test runner image registry secret used to pull/push images
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ImageRegistrySecret",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:imageRegistrySecret"}
	// +optional
	ImageRegistrySecret *v1.LocalObjectReference `json:"imageRegistrySecret,omitempty"`

	// config map to customize the config for test runner, if not specified default test config will be aplied
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Secret",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:configmap"}
	// +optional
	Config *v1.LocalObjectReference `json:"config,omitempty"`

	// Selector describes on which nodes to enable test runner
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Selector",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:selector"}
	// +optional
	Selector map[string]string `json:"selector,omitempty"`

	// upgrade policy for test runner daemonset
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="UpgradePolicy",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:upgradePolicy"}
	// +optional
	UpgradePolicy *DaemonSetUpgradeSpec `json:"upgradePolicy,omitempty"`

	// captures logs location and export config for test runner logs
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="LogsLocation",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:logsLocation"}
	// +optional
	LogsLocation LogsLocationConfig `json:"logsLocation,omitempty"`
}

// LogsLocationConfig contains mount and export config for test runner logs
type LogsLocationConfig struct {
	// volume mount destination within test runner container
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="MountPath",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:mountPath"}
	// +kubebuilder:default="/var/log/amd-test-runner"
	// +optional
	MountPath string `json:"mountPath,omitempty"`

	// host path to store test runner internal status db in order to persist test running status
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="HostPath",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:hostPath"}
	// +kubebuilder:default="/var/log/amd-test-runner"
	// +optional
	HostPath string `json:"hostPath,omitempty"`

	// LogsExportSecrets is a list of secrets that contain connectivity info to multiple cloud providers
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="LogsExportSecrets",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:logsExportSecrets"}
	// +optional
	LogsExportSecrets []*v1.LocalObjectReference `json:"logsExportSecrets,omitempty"`
}

// UtilsContainerSpec contains parameters to configure operator's utils
type UtilsContainerSpec struct {
	// Image is the image of utils container
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Image",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:image"}
	// +optional
	// +kubebuilder:validation:Pattern=`^([a-z0-9]+(?:[._-][a-z0-9]+)*(:[0-9]+)?)(/[a-z0-9]+(?:[._-][a-z0-9]+)*)*(?::[a-z0-9._-]+)?(?:@[a-zA-Z0-9]+:[a-f0-9]+)?$`
	Image string `json:"image,omitempty"`

	// image pull policy for utils container
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ImagePullPolicy",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:imagePullPolicy"}
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	// +optional
	ImagePullPolicy string `json:"imagePullPolicy,omitempty"`

	// secret used for pull utils container image
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ImageRegistrySecret",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:imageRegistrySecret"}
	// +optional
	ImageRegistrySecret *v1.LocalObjectReference `json:"imageRegistrySecret,omitempty"`
}

// CommonConfigSpec contains the common config across operator and operands
type CommonConfigSpec struct {
	// InitContainerImage is being used for the operands pods, i.e. metrics exporter, test runner, device plugin and node labeller
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="InitContainerImage",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:initContainerImage"}
	// +optional
	InitContainerImage string `json:"initContainerImage,omitempty"`

	// UtilsContainer contains parameters to configure operator's utils container
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="UtilsContainer",xDescriptors={"urn:alm:descriptor:com.amd.NetworkConfigs:utilsContainer"}
	// +optional
	UtilsContainer UtilsContainerSpec `json:"utilsContainer,omitempty"`
}

// DeploymentStatus contains the status for a daemonset deployed during
// reconciliation loop
type DeploymentStatus struct {
	// number of nodes that are targeted by the NetworkConfig selector
	//+operator-sdk:csv:customresourcedefinitions:type=status,displayName="NodesMatchingSelectorNumber",xDescriptors="urn:alm:descriptor:com.amd.NetworkConfigs:nodesMatchingSelectorNumber"
	NodesMatchingSelectorNumber int32 `json:"nodesMatchingSelectorNumber,omitempty"`
	// number of the pods that should be deployed for daemonset
	//+operator-sdk:csv:customresourcedefinitions:type=status,displayName="DesiredNumber",xDescriptors="urn:alm:descriptor:com.amd.NetworkConfigs:desiredNumber"
	DesiredNumber int32 `json:"desiredNumber,omitempty"`
	// number of the actually deployed and running pods
	//+operator-sdk:csv:customresourcedefinitions:type=status,displayName="AvailableNumber",xDescriptors="urn:alm:descriptor:com.amd.NetworkConfigs:availableNumber"
	AvailableNumber int32 `json:"availableNumber,omitempty"`
}

// ModuleStatus contains the status of driver module installed by operator on the node
type ModuleStatus struct {
	ContainerImage     string       `json:"containerImage,omitempty"`
	KernelVersion      string       `json:"kernelVersion,omitempty"`
	LastTransitionTime string       `json:"lastTransitionTime,omitempty"`
	Status             UpgradeState `json:"status,omitempty"`
	UpgradeStartTime   string       `json:"upgradeStartTime,omitempty"`
	BootId             string       `json:"bootId,omitempty"`
}

// NetworkConfigStatus defines the observed state of Module.
type NetworkConfigStatus struct {
	// DevicePlugin contains the status of the Device Plugin deployment
	DevicePlugin DeploymentStatus `json:"devicePlugin,omitempty"`
	// Driver contains the status of the Drivers deployment
	Drivers DeploymentStatus `json:"driver,omitempty"`
	// MetricsExporter contains the status of the MetricsExporter deployment
	MetricsExporter DeploymentStatus `json:"metricsExporter,omitempty"`
	// ConfigManager contains the status of the ConfigManager deployment
	ConfigManager DeploymentStatus `json:"configManager,omitempty"`
	// NodeModuleStatus contains per node status of driver module installation
	//+operator-sdk:csv:customresourcedefinitions:type=status,displayName="NodeModuleStatus",xDescriptors="urn:alm:descriptor:com.amd.NetworkConfigs:nodeModuleStatus"
	NodeModuleStatus map[string]ModuleStatus `json:"nodeModuleStatus,omitempty"`
	// Conditions list the current status of the NetworkConfig object
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// ObservedGeneration is the latest spec generation successfully processed by the controller
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Namespaced,shortName=nwcfg
//+kubebuilder:subresource:status

// NetworkConfig describes how to enable AMD Network device
// +operator-sdk:csv:customresourcedefinitions:displayName="NetworkConfig",resources={{Module,v1beta1,modules.kmm.sigs.x-k8s.io},{Daemonset,v1,apps}, {services,v1,core},{Pod,v1,core}}
type NetworkConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkConfigSpec   `json:"spec,omitempty"`
	Status NetworkConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NetworkConfigList contains a list of NetworkConfigs
type NetworkConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NetworkConfig{}, &NetworkConfigList{})
}
