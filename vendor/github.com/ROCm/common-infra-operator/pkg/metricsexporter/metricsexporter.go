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

package metricsexporter

import (
	"encoding/json"
	"fmt"

	protos "github.com/ROCm/common-infra-operator/pkg/protos"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

const (
	defaultKubeRbacProxyImage = "quay.io/brancz/kube-rbac-proxy:v0.18.1"
	nobodyUser                = 65532
	ExporterName              = "metrics-exporter"
	kubeRBACName              = "kube-rbac-proxy"
	StaticAuthSecretName      = ExporterName + "-static-auth-config"
)

//go:generate mockgen -source=metricsexporter.go -package=metricsexporter -destination=mock_metricsexporter.go MetricsExporter
type MetricsExporter interface {
	SetMetricsExporterAsDesired(ds *appsv1.DaemonSet, metricsConfig *protos.MetricsExporterSpec) (*runtime.Scheme, error)
	SetMetricsServiceAsDesired(svc *v1.Service, metricsConfig *protos.MetricsExporterSpec) (*runtime.Scheme, error)
	SetStaticAuthSecretAsDesired(secret *v1.Secret, metricsConfig *protos.MetricsExporterSpec) (*runtime.Scheme, error)
	SetServiceMonitorAsDesired(sm *monitoringv1.ServiceMonitor, metricsConfig *protos.MetricsExporterSpec) (*runtime.Scheme, error)
}

type metricsExporter struct {
	scheme *runtime.Scheme
}

func NewMetricsExporter(scheme *runtime.Scheme) MetricsExporter {
	return &metricsExporter{
		scheme: scheme,
	}
}

func (exp *metricsExporter) SetMetricsExporterAsDesired(ds *appsv1.DaemonSet, expSpec *protos.MetricsExporterSpec) (*runtime.Scheme, error) {
	if ds == nil {
		return exp.scheme, fmt.Errorf("daemon set is not initialized, zero pointer")
	}

	nodeSelector := expSpec.DsSpec.Selector

	mainContainerImage := expSpec.DsSpec.MainContainer.Image
	if expSpec.DsSpec.MainContainer.Image == "" {
		mainContainerImage = expSpec.DsSpec.MainContainer.DefaultImage
	}
	initContainerImage := expSpec.DsSpec.InitContainers[0].Image
	if expSpec.DsSpec.InitContainers[0].Image == "" {
		initContainerImage = expSpec.DsSpec.InitContainers[0].DefaultImage
	}

	port := expSpec.SvcSpec.ServicePort
	if expSpec.SvcSpec.Port > 0 {
		port = expSpec.SvcSpec.Port
	}

	containers := []v1.Container{
		{
			Name:            ExporterName + "-container",
			Image:           mainContainerImage,
			WorkingDir:      "/root",
			SecurityContext: &v1.SecurityContext{Privileged: ptr.To(expSpec.DsSpec.MainContainer.IsPrivileged)},
			Env:             expSpec.DsSpec.MainContainer.Envs,
			VolumeMounts:    expSpec.DsSpec.MainContainer.VolumeMounts,
			Command:         expSpec.DsSpec.MainContainer.Command,
			Args:            expSpec.DsSpec.MainContainer.Arguments,
		},
	}

	if expSpec.DsSpec.MainContainer.ImagePullPolicy != "" {
		containers[0].ImagePullPolicy = v1.PullPolicy(expSpec.DsSpec.MainContainer.ImagePullPolicy)
	}

	imagePullSecrets := []v1.LocalObjectReference{}
	if expSpec.DsSpec.MainContainer.ImageRegistrySecret != nil {
		imagePullSecrets = append(imagePullSecrets, *expSpec.DsSpec.MainContainer.ImageRegistrySecret)
	}

	if expSpec.RbacConfig.Enable != nil && *expSpec.RbacConfig.Enable {
		internalPort := expSpec.SvcSpec.ServicePort
		if internalPort == port {
			internalPort = port - 1
		}
		// Bind service port to localhost only, don't expose port in ContainerPort
		if containers[0].Args == nil {
			containers[0].Args = []string{}
		}
		containers[0].Args = append(containers[0].Args, "--bind=127.0.0.1")
		// Find and update METRICS_EXPORTER_PORT environment variable
		found := false
		for i, env := range containers[0].Env {
			if env.Name == "METRICS_EXPORTER_PORT" {
				containers[0].Env[i].Value = fmt.Sprintf("%v", internalPort)
				found = true
				break
			}
		}
		if !found {
			if containers[0].Env == nil {
				containers[0].Env = []v1.EnvVar{}
			}
			containers[0].Env = append(containers[0].Env, v1.EnvVar{
				Name:  "METRICS_EXPORTER_PORT",
				Value: fmt.Sprintf("%v", internalPort),
			})
		}

		kubeImage := defaultKubeRbacProxyImage
		if expSpec.RbacConfig.Image != "" {
			kubeImage = expSpec.RbacConfig.Image
		}

		args := []string{
			"--upstream=http://127.0.0.1:" + fmt.Sprintf("%v", int32(internalPort)),
			"--logtostderr=true",
			"--v=10",
		}

		volumeMounts := []v1.VolumeMount{}
		// Add client CA config map mount for mTLS if specified
		if expSpec.RbacConfig.ClientCAConfigMap != nil {
			expSpec.DsSpec.Volumes = append(expSpec.DsSpec.Volumes, v1.Volume{
				Name: "client-ca",
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: *expSpec.RbacConfig.ClientCAConfigMap,
					},
				},
			})
			volumeMounts = append(volumeMounts, v1.VolumeMount{
				Name:      "client-ca",
				MountPath: "/etc/kube-rbac-proxy/ca",
				ReadOnly:  true,
			})
			args = append(args, "--client-ca-file=/etc/kube-rbac-proxy/ca/ca.crt")
		}

		// Create and mount static authorization config if enabled
		if expSpec.RbacConfig.StaticAuthorization != nil && expSpec.RbacConfig.StaticAuthorization.Enable {
			// Add volume and mount for static auth config
			expSpec.DsSpec.Volumes = append(expSpec.DsSpec.Volumes, v1.Volume{
				Name: "static-auth-config",
				VolumeSource: v1.VolumeSource{
					Secret: &v1.SecretVolumeSource{
						SecretName: expSpec.RbacConfig.StaticAuthorization.SecretName,
					},
				},
			})
			volumeMounts = append(volumeMounts, v1.VolumeMount{
				Name:      "static-auth-config",
				MountPath: "/etc/kube-rbac-proxy",
				ReadOnly:  true,
			})
			args = append(args, "--config-file=/etc/kube-rbac-proxy/config.yaml")
		}

		// Continue with existing TLS cert handling
		if expSpec.RbacConfig.DisableHttps != nil && *expSpec.RbacConfig.DisableHttps {
			args = append(args, "--insecure-listen-address=0.0.0.0:"+fmt.Sprintf("%v", int32(port)))
		} else {
			args = append(args, "--secure-listen-address=0.0.0.0:"+fmt.Sprintf("%v", int32(port)))

			// Load the tls-certs if provided
			if expSpec.RbacConfig.Secret != nil {
				volumeMounts = append(volumeMounts, v1.VolumeMount{
					Name:      "tls-certs",
					MountPath: "/etc/tls",
					ReadOnly:  true,
				})

				args = append(args, "--tls-cert-file=/etc/tls/tls.crt")
				args = append(args, "--tls-private-key-file=/etc/tls/tls.key")
			}
		}

		containers = append(containers, v1.Container{
			Name:  kubeRBACName + "-container",
			Image: kubeImage,
			SecurityContext: &v1.SecurityContext{
				RunAsUser:                ptr.To(int64(nobodyUser)),
				AllowPrivilegeEscalation: ptr.To(false),
			},
			Args:         args,
			VolumeMounts: volumeMounts,
			Ports: []v1.ContainerPort{
				{
					Name:          "exporter-port",
					Protocol:      v1.ProtocolTCP,
					ContainerPort: port,
				},
			},
		})
	} else {
		containers[0].Env[1].Value = fmt.Sprintf("%v", port)
		containers[0].Ports = []v1.ContainerPort{
			{
				Name:          "exporter-port",
				Protocol:      v1.ProtocolTCP,
				ContainerPort: port,
			},
		}
	}

	gracePeriod := int64(1)
	ds.Spec = appsv1.DaemonSetSpec{
		Selector: &metav1.LabelSelector{MatchLabels: expSpec.DsSpec.Labels},
		Template: v1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: expSpec.DsSpec.Labels,
			},

			Spec: v1.PodSpec{
				InitContainers: []v1.Container{
					{
						Name:            "driver-init",
						Image:           initContainerImage,
						Command:         expSpec.DsSpec.InitContainers[0].Command,
						SecurityContext: &v1.SecurityContext{Privileged: ptr.To(expSpec.DsSpec.InitContainers[0].IsPrivileged)},
						Env:             expSpec.DsSpec.InitContainers[0].Envs,
						VolumeMounts:    expSpec.DsSpec.InitContainers[0].VolumeMounts,
					},
				},
				Containers:                    containers,
				PriorityClassName:             "system-node-critical",
				NodeSelector:                  nodeSelector,
				ServiceAccountName:            expSpec.DsSpec.ServiceAccountName,
				Volumes:                       expSpec.DsSpec.Volumes,
				ImagePullSecrets:              imagePullSecrets,
				TerminationGracePeriodSeconds: &gracePeriod,
				HostNetwork:                   expSpec.DsSpec.MainContainer.IsHostNetwork,
			},
		},
	}
	if expSpec.DsSpec.UpgradePolicy != nil {
		up := expSpec.DsSpec.UpgradePolicy
		upgradeStrategy := appsv1.RollingUpdateDaemonSetStrategyType
		if up.UpgradeStrategy == "OnDelete" {
			upgradeStrategy = appsv1.OnDeleteDaemonSetStrategyType
		}
		ds.Spec.UpdateStrategy = appsv1.DaemonSetUpdateStrategy{
			Type: upgradeStrategy,
		}
		if upgradeStrategy == appsv1.RollingUpdateDaemonSetStrategyType {
			ds.Spec.UpdateStrategy.RollingUpdate = &appsv1.RollingUpdateDaemonSet{
				MaxUnavailable: &intstr.IntOrString{IntVal: int32(up.MaxUnavailable)},
			}
		}
	}
	if len(expSpec.DsSpec.Tolerations) > 0 {
		ds.Spec.Template.Spec.Tolerations = expSpec.DsSpec.Tolerations
	} else {
		ds.Spec.Template.Spec.Tolerations = nil
	}

	return exp.scheme, nil
}

func (exp *metricsExporter) SetMetricsServiceAsDesired(svc *v1.Service, expSpec *protos.MetricsExporterSpec) (*runtime.Scheme, error) {
	if svc == nil {
		return exp.scheme, fmt.Errorf("service  is not initialized, zero pointer")
	}

	// Add app label for ServiceMonitor selection
	svc.Labels = expSpec.SvcSpec.Labels

	svc.Spec = v1.ServiceSpec{
		Selector: expSpec.DsSpec.Labels,
	}

	port := expSpec.SvcSpec.ServicePort
	if expSpec.SvcSpec.Port > 0 {
		port = expSpec.SvcSpec.Port
	}

	trafficPolicyLocal := v1.ServiceInternalTrafficPolicyLocal
	svc.Spec.InternalTrafficPolicy = &trafficPolicyLocal

	switch expSpec.SvcSpec.SvcType {
	case protos.ServiceTypeNodePort:
		svc.Spec.Type = v1.ServiceTypeNodePort
		svc.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyLocal
		svc.Spec.Ports = []v1.ServicePort{
			{
				Name:       "exporter-port",
				Protocol:   v1.ProtocolTCP,
				Port:       port,
				TargetPort: intstr.FromInt32(port),
				NodePort:   expSpec.SvcSpec.NodePort,
			},
		}
	default:
		svc.Spec.Type = v1.ServiceTypeClusterIP
		svc.Spec.Ports = []v1.ServicePort{
			{
				Name:       "exporter-port",
				Protocol:   v1.ProtocolTCP,
				Port:       port,
				TargetPort: intstr.FromInt32(port),
			},
		}

	}

	return exp.scheme, nil
}

// SetServiceMonitorAsDesired configures the ServiceMonitor resource for Prometheus integration
// Ignoring staticcheck linter for this function. SA1019: we intentionally use BearerTokenFile, a deprecated field for compatibility
//
//nolint:staticcheck
func (exp *metricsExporter) SetServiceMonitorAsDesired(sm *monitoringv1.ServiceMonitor, expSpec *protos.MetricsExporterSpec) (*runtime.Scheme, error) {
	if sm == nil {
		return exp.scheme, fmt.Errorf("ServiceMonitor is not initialized, zero pointer")
	}

	// Configure app label selector for the service
	labelSelector := metav1.LabelSelector{
		MatchLabels: expSpec.SvcSpec.Labels,
	}

	port := expSpec.SvcSpec.ServicePort
	if expSpec.SvcSpec.Port > 0 {
		port = expSpec.SvcSpec.Port
	}

	// Set up the endpoint
	endpoints := []monitoringv1.Endpoint{
		{
			Port:                 "exporter-port",
			TargetPort:           &intstr.IntOrString{Type: intstr.Int, IntVal: port},
			RelabelConfigs:       []monitoringv1.RelabelConfig{},
			MetricRelabelConfigs: []monitoringv1.RelabelConfig{},
		},
	}

	// Apply custom interval if specified
	if expSpec.Prometheus.ServiceMonitor.Interval != "" {
		endpoints[0].Interval = monitoringv1.Duration(expSpec.Prometheus.ServiceMonitor.Interval)
	}

	// Apply honorLabels if specified
	if expSpec.Prometheus.ServiceMonitor.HonorLabels != nil {
		endpoints[0].HonorLabels = *expSpec.Prometheus.ServiceMonitor.HonorLabels
	} else {
		endpoints[0].HonorLabels = false
	}

	// Apply honorTimestamps if specified
	if expSpec.Prometheus.ServiceMonitor.HonorTimestamps != nil {
		endpoints[0].HonorTimestamps = expSpec.Prometheus.ServiceMonitor.HonorTimestamps
	}

	// Apply relabelings if specified
	if len(expSpec.Prometheus.ServiceMonitor.Relabelings) > 0 {
		endpoints[0].RelabelConfigs = expSpec.Prometheus.ServiceMonitor.Relabelings
	}

	// Apply metricRelabelings if specified
	if len(expSpec.Prometheus.ServiceMonitor.MetricRelabelings) > 0 {
		endpoints[0].MetricRelabelConfigs = expSpec.Prometheus.ServiceMonitor.MetricRelabelings
	}

	// Default scheme to http
	endpoints[0].Scheme = "http"

	// Use HTTPS when RBAC is enabled and HTTPS is not explicitly disabled
	if expSpec.RbacConfig.Enable != nil &&
		*expSpec.RbacConfig.Enable {
		// If DisableHttps is nil or false, use HTTPS
		if expSpec.RbacConfig.DisableHttps == nil ||
			!*expSpec.RbacConfig.DisableHttps {
			endpoints[0].Scheme = "https"
		}
		// Set TLS config for HTTPS
		if expSpec.Prometheus.ServiceMonitor.TLSConfig != nil {
			endpoints[0].TLSConfig = expSpec.Prometheus.ServiceMonitor.TLSConfig
		}

		// Set bearer token file for RBAC proxy
		if expSpec.Prometheus.ServiceMonitor.BearerTokenFile != "" {
			endpoints[0].BearerTokenFile = expSpec.Prometheus.ServiceMonitor.BearerTokenFile
		}

		// Set Authorization if specified
		if expSpec.Prometheus.ServiceMonitor.Authorization != nil {
			endpoints[0].Authorization = expSpec.Prometheus.ServiceMonitor.Authorization
		}
	}

	// Configure ServiceMonitor
	sm.Spec = monitoringv1.ServiceMonitorSpec{
		Selector:          labelSelector,
		Endpoints:         endpoints,
		NamespaceSelector: monitoringv1.NamespaceSelector{MatchNames: []string{expSpec.DsSpec.Namespace}},
		AttachMetadata:    expSpec.Prometheus.ServiceMonitor.AttachMetadata,
	}

	// Set custom labels
	sm.Labels = expSpec.Prometheus.ServiceMonitor.Labels

	return exp.scheme, nil
}

// SetStaticAuthSecretAsDesired creates a secret containing the kube-rbac-proxy static authorization config
func (exp *metricsExporter) SetStaticAuthSecretAsDesired(secret *v1.Secret, expSpec *protos.MetricsExporterSpec) (*runtime.Scheme, error) {
	if secret == nil {
		return exp.scheme, fmt.Errorf("secret is not initialized, zero pointer")
	}

	if expSpec.RbacConfig.StaticAuthorization == nil || !expSpec.RbacConfig.StaticAuthorization.Enable {
		return exp.scheme, nil
	}

	staticAuthConfig := map[string]interface{}{
		"authorization": map[string]interface{}{
			"static": []map[string]interface{}{
				{
					"path":            "/metrics",
					"resourceRequest": false,
					"user": map[string]string{
						"name": expSpec.RbacConfig.StaticAuthorization.ClientName,
					},
					"verb": "get",
				},
			},
		},
	}

	staticAuthConfigJSON, err := json.Marshal(staticAuthConfig)
	if err != nil {
		return exp.scheme, fmt.Errorf("failed to marshal static auth config: %v", err)
	}

	secret.StringData = map[string]string{
		"config.yaml": string(staticAuthConfigJSON),
	}

	return exp.scheme, nil
}
