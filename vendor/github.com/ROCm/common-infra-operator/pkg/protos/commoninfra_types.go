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

package protos

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	v1 "k8s.io/api/core/v1"
)

// ServiceType string describes ingress methods for a service
type ServiceType string

const (
	// ServiceTypeClusterIP to access inside the cluster
	ServiceTypeClusterIP ServiceType = "ClusterIP"

	// ServiceTypeNodePort to expose service to external
	ServiceTypeNodePort ServiceType = "NodePort"
)

type CommonContainerSpec struct {
	DefaultImage        string                   `json:"defaultImage,omitempty"`
	DefaultUbiImage     string                   `json:"defaultUbiImage,omitempty"`
	Image               string                   `json:"image,omitempty"`
	ImagePullPolicy     string                   `json:"imagePullPolicy,omitempty"`
	ImageRegistrySecret *v1.LocalObjectReference `json:"imageRegistrySecret,omitempty"`
	IsPrivileged        bool                     `json:"isPrivileged,omitempty"`
	IsHostNetwork       bool                     `json:"isHostNetwork,omitempty"`
	Command             []string                 `json:"command,omitempty"`
	Arguments           []string                 `json:"arguments,omitempty"`
	Envs                []v1.EnvVar              `json:"envs,omitempty"`
	VolumeMounts        []v1.VolumeMount         `json:"volumeMounts,omitempty"`
	Labels              map[string]string        `json:"labels,omitempty"`
}

type InitContainerSpec struct {
	CommonContainerSpec
}
type MainContainerSpec struct {
	CommonContainerSpec
}

type ServiceSpec struct {
	ServicePort int32             `json:"servicePort,omitempty"` //internal port used (default 5000)
	Port        int32             `json:"port,omitempty"`        // port used for in-cluster and to pull metrics (default 5000)
	NodePort    int32             `json:"nodePort,omitempty"`    // port usef from outside cluster to pull metrics, in range 30000-32767
	SvcType     ServiceType       `json:"serviceType,omitempty"` // ClusterIP/NOdePort , clusterIP by default
	Labels      map[string]string `json:"labels,omitempty"`      // labels to add to the service
}

type DaemonSetOperandSpec struct {
	Name               string                `json:"name,omitempty"`
	Namespace          string                `json:"namespace,omitempty"`
	Enable             *bool                 `json:"enable,omitempty"`
	ServiceAccountName string                `json:"serviceAccountName,omitempty"`
	Tolerations        []v1.Toleration       `json:"tolerations,omitempty"`
	UpgradePolicy      *DaemonSetUpgradeSpec `json:"upgradePolicy,omitempty"`
	Selector           map[string]string     `json:"selector,omitempty"`
	InitContainers     []InitContainerSpec   `json:"initContainers,omitempty"`
	MainContainer      MainContainerSpec     `json:"mainContainer,omitempty"`
	Volumes            []v1.Volume           `json:"volumes,omitempty"`
	Labels             map[string]string     `json:"labels,omitempty"`
}

type DevicePluginSpec struct {
	DaemonSetOperandSpec
}

type MetricsExporterSpec struct {
	DsSpec     DaemonSetOperandSpec `json:"dsSpec,omitempty"`
	SvcSpec    ServiceSpec          `json:"svcSpec,omitempty"`
	Config     MetricsConfig        `json:"config,omitempty"`     // +optional
	RbacConfig KubeRbacConfig       `json:"rbacConfig,omitempty"` // +optional
	Prometheus *PrometheusConfig    `json:"prometheus,omitempty"` // +optional
}

type NodeLabellerSpec struct {
	DaemonSetOperandSpec
}

type PrometheusConfig struct {
	// ServiceMonitor configuration for Prometheus integration
	// +optional
	ServiceMonitor *ServiceMonitorConfig `json:"serviceMonitor,omitempty"`
}

// ServiceMonitorConfig provides configuration for ServiceMonitor
type ServiceMonitorConfig struct {
	// Enable or disable ServiceMonitor creation (default false)
	// +optional
	Enable *bool `json:"enable,omitempty"`

	// How frequently to scrape metrics. Accepts values with time unit suffix: "30s", "1m", "2h", "500ms"
	// +optional
	Interval string `json:"interval,omitempty"`

	// AttachMetadata defines if Prometheus should attach node metadata to the target
	// +optional
	AttachMetadata *monitoringv1.AttachMetadata `json:"attachMetadata,omitempty"`

	// HonorLabels chooses the metric's labels on collisions with target labels (default false)
	// +optional
	HonorLabels *bool `json:"honorLabels,omitempty"`

	// HonorTimestamps controls whether the scrape endpoints honor timestamps (default false)
	// +optional
	HonorTimestamps *bool `json:"honorTimestamps,omitempty"`

	// Additional labels to add to the ServiceMonitor (default release: prometheus)
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// RelabelConfigs to apply to samples before ingestion
	// +optional
	Relabelings []monitoringv1.RelabelConfig `json:"relabelings,omitempty"`

	// Relabeling rules applied to individual scraped metrics
	// +optional
	MetricRelabelings []monitoringv1.RelabelConfig `json:"metricRelabelings,omitempty"`

	// Optional Prometheus authorization configuration for accessing the endpoint
	// +optional
	Authorization *monitoringv1.SafeAuthorization `json:"authorization,omitempty"`

	// Path to bearer token file to be used by Prometheus (e.g., service account token path)
	// Deprecated: Use Authorization instead. This field is kept for backward compatibility.
	// +optional
	BearerTokenFile string `json:"bearerTokenFile,omitempty"`

	// TLS settings used by Prometheus to connect to the metrics endpoint
	// +optional
	TLSConfig *monitoringv1.TLSConfig `json:"tlsConfig,omitempty"`
}

// KubeRbacConfig contains configs for kube-rbac-proxy sidecar
type KubeRbacConfig struct {
	// enable kube-rbac-proxy, disabled by default
	// +optional
	Enable *bool `json:"enable,omitempty"`

	// kube-rbac-proxy image
	// +optional
	Image string `json:"image,omitempty"`

	// disable https protecting the proxy endpoint
	// +optional
	DisableHttps *bool `json:"disableHttps,omitempty"`

	// certificate secret to mount in kube-rbac container for TLS, self signed certificates will be generated by default
	// +optional
	Secret *v1.LocalObjectReference `json:"secret,omitempty"`

	// Reference to a configmap containing the client CA (key: ca.crt) for mTLS client validation
	// +optional
	ClientCAConfigMap *v1.LocalObjectReference `json:"clientCAConfigMap,omitempty"`

	// Optional static RBAC rules based on client certificate Common Name (CN)
	// +optional
	StaticAuthorization *StaticAuthConfig `json:"staticAuthorization,omitempty"`
}

// StaticAuthConfig contains static authorization configuration for kube-rbac-proxy
type StaticAuthConfig struct {
	// Enables static authorization using client certificate CN
	Enable bool `json:"enable,omitempty"`

	// Expected CN (Common Name) from client cert (e.g., Prometheus SA identity)
	ClientName string `json:"clientName,omitempty"`

	// Static Auth secret name to be mounted in the exporter pod
	SecretName string `json:"secretName,omitempty"`
}

// MetricsConfig contains list of metrics to collect/report
type MetricsConfig struct {
	// Name of the configMap that defines the list of metrics
	// default list:[]
	// +optional
	Name string `json:"name,omitempty"`
}

type DaemonSetUpgradeSpec struct {
	// UpgradeStrategy specifies the type of the DaemonSet update. Valid values are "RollingUpdate" (default) or "OnDelete".
	// +optional
	UpgradeStrategy string `json:"upgradeStrategy,omitempty"`

	// MaxUnavailable specifies the maximum number of Pods that can be unavailable during the update process. Applicable for RollingUpdate only. Default value is 1.
	MaxUnavailable int32 `json:"maxUnavailable,omitempty"`
}
