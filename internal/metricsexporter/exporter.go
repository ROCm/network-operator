/*
Copyright (c) 2025 Advanced Micro Devices, Inc. All rights reserved.

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

package exporterinternal

import (
	"fmt"
	"os"
	"strconv"

	v1 "k8s.io/api/core/v1"

	protos "github.com/ROCm/common-infra-operator/pkg/protos"
	amdv1alpha1 "github.com/ROCm/network-operator/api/v1alpha1"
	utils "github.com/ROCm/network-operator/internal"
)

const (
	exporterServicePort         int32 = 5001
	ExporterName                      = "metrics-exporter"
	exporterSAName                    = "amd-network-operator-metrics-exporter"
	kubeRBACSAName                    = "amd-network-operator-metrics-exporter-rbac-proxy"
	StaticAuthSecretName              = ExporterName + "-static-auth-config"
	defaultMetricsExporterImage       = "docker.io/rocm/device-metrics-exporter:nic-v1.0.1"
	defaultInitContainerImage         = "busybox:1.36"
	svcLabel                          = "app.kubernetes.io/service"
)

func GenerateCommonExporterSpec(nwConfig *amdv1alpha1.NetworkConfig) *protos.MetricsExporterSpec {
	var specOut protos.MetricsExporterSpec
	specIn := &nwConfig.Spec.MetricsExporter
	simEnabled, _ := strconv.ParseBool(os.Getenv("SIM_ENABLE"))

	specOut.DsSpec.Name = fmt.Sprintf("%s-%s", nwConfig.Name, ExporterName)
	specOut.DsSpec.Namespace = nwConfig.Namespace
	specOut.DsSpec.Enable = specIn.Enable
	specOut.DsSpec.ServiceAccountName = exporterSAName
	specOut.DsSpec.Tolerations = specIn.Tolerations
	specOut.DsSpec.UpgradePolicy = (*protos.DaemonSetUpgradeSpec)(specIn.UpgradePolicy.DeepCopy())

	specOut.SvcSpec.Port = specIn.Port
	specOut.SvcSpec.SvcType = protos.ServiceType(specIn.SvcType)
	specOut.SvcSpec.NodePort = specIn.NodePort
	specOut.SvcSpec.ServicePort = exporterServicePort
	specOut.Config = protos.MetricsConfig(specIn.Config)
	specOut.RbacConfig = protos.KubeRbacConfig{
		Enable:            specIn.RbacConfig.Enable,
		Image:             specIn.RbacConfig.Image,
		DisableHttps:      specIn.RbacConfig.DisableHttps,
		Secret:            specIn.RbacConfig.Secret,
		ClientCAConfigMap: specIn.RbacConfig.ClientCAConfigMap,
	}
	if specIn.RbacConfig.StaticAuthorization != nil {
		specOut.RbacConfig.StaticAuthorization = &protos.StaticAuthConfig{
			Enable:     specIn.RbacConfig.StaticAuthorization.Enable,
			ClientName: specIn.RbacConfig.StaticAuthorization.ClientName,
			SecretName: nwConfig.Name + "-" + StaticAuthSecretName,
		}
	}

	serviceMonitorLabelPair := []string{"app", "amd-device-metrics-exporter"}
	// Copy Prometheus configuration if present
	if specIn.Prometheus != nil {
		specOut.Prometheus = &protos.PrometheusConfig{}
		if specIn.Prometheus.ServiceMonitor != nil {
			smIn := specIn.Prometheus.ServiceMonitor
			smOut := &protos.ServiceMonitorConfig{
				Enable:            smIn.Enable,
				Interval:          smIn.Interval,
				AttachMetadata:    smIn.AttachMetadata,
				HonorLabels:       smIn.HonorLabels,
				HonorTimestamps:   smIn.HonorTimestamps,
				Labels:            map[string]string{},
				Relabelings:       smIn.Relabelings,
				MetricRelabelings: smIn.MetricRelabelings,
				Authorization:     smIn.Authorization,
				BearerTokenFile:   smIn.BearerTokenFile,
				TLSConfig:         smIn.TLSConfig,
			}

			// Copy user-supplied labels
			for k, v := range smIn.Labels {
				smOut.Labels[k] = v
			}
			// Add default "app" label if user did not override
			if _, exists := smOut.Labels["app"]; !exists {
				smOut.Labels[serviceMonitorLabelPair[0]] = serviceMonitorLabelPair[1]
			}

			specOut.Prometheus.ServiceMonitor = smOut
		}
	}

	metricsExporterLabelPair := []string{"app.kubernetes.io/name", ExporterName}
	specOut.DsSpec.Labels = map[string]string{
		// TODO: Do we need a CR label?
		utils.CRNameLabel:           nwConfig.Name,
		"daemonset-name":            nwConfig.Name + "-" + ExporterName,
		metricsExporterLabelPair[0]: metricsExporterLabelPair[1],
	}

	specOut.SvcSpec.Labels = map[string]string{
		svcLabel: nwConfig.Name + "-" + ExporterName,
	}

	if specIn.Selector != nil {
		specOut.DsSpec.Selector = specIn.Selector
	} else if nwConfig.Spec.Selector != nil {
		specOut.DsSpec.Selector = nwConfig.Spec.Selector
	}
	if specOut.RbacConfig.Enable != nil && *specOut.RbacConfig.Enable {
		specOut.DsSpec.ServiceAccountName = kubeRBACSAName // elevated privilege when rbac-proxy is enabled
	}

	specOut.DsSpec.InitContainers = make([]protos.InitContainerSpec, 1)
	specOut.DsSpec.InitContainers[0].DefaultImage = defaultInitContainerImage
	specOut.DsSpec.InitContainers[0].Image = nwConfig.Spec.CommonConfig.InitContainerImage
	if simEnabled {
		specOut.DsSpec.InitContainers[0].Command = []string{}
	} else {
		specOut.DsSpec.InitContainers[0].Command = []string{
			"sh", "-c",
			`while [ ! -d /host-sys/class/infiniband ] || 
			       [ ! -d /host-sys/class/infiniband_verbs ] || 
			       [ ! -d /host-sys/module/ionic/drivers ]; do 
		        echo "amd ionic driver is not loaded " 
			    sleep 2 
			done`,
		}
	}
	specOut.DsSpec.InitContainers[0].Envs = []v1.EnvVar{
		{
			Name:  "SIM_ENABLE",
			Value: os.Getenv("SIM_ENABLE"),
		},
	}
	specOut.DsSpec.InitContainers[0].VolumeMounts = []v1.VolumeMount{
		{
			Name:      "sys-volume",
			MountPath: "/host-sys",
		},
	}

	specOut.DsSpec.MainContainer.DefaultImage = defaultMetricsExporterImage
	specOut.DsSpec.MainContainer.Image = specIn.Image
	specOut.DsSpec.MainContainer.ImagePullPolicy = specIn.ImagePullPolicy
	specOut.DsSpec.MainContainer.ImageRegistrySecret = specIn.ImageRegistrySecret
	specOut.DsSpec.MainContainer.IsPrivileged = true
	specOut.DsSpec.MainContainer.IsHostNetwork = *specIn.HostNetwork
	specOut.DsSpec.MainContainer.Arguments = []string{"-monitor-nic=true", "-monitor-gpu=false"}

	// Exporter Specifc Values
	specOut.DsSpec.MainContainer.Envs = []v1.EnvVar{
		{
			Name: "DS_NODE_NAME",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "spec.nodeName",
				},
			},
		},
		{
			Name: "METRICS_EXPORTER_PORT",
		},
	}

	hostPathDirectory := v1.HostPathDirectory
	hostPathDirectoryOrCreate := v1.HostPathDirectoryOrCreate
	healthCreateHostDirectory := v1.HostPathDirectoryOrCreate
	specOut.DsSpec.MainContainer.VolumeMounts = []v1.VolumeMount{
		{
			Name:      "dev-volume",
			MountPath: "/dev",
		},
		{
			Name:      "sys-volume",
			MountPath: "/sys",
		},
		{
			Name:      "pod-resources",
			MountPath: "/var/lib/kubelet/pod-resources",
		},
		{
			Name:      "health",
			MountPath: "/var/lib/amd-metrics-exporter",
		},
		{
			Name:      "slurm",
			MountPath: "/var/run/exporter",
		},
	}
	nonSimMounts := []v1.VolumeMount{
		{
			Name:      "proc",
			MountPath: "/host/proc",
		},
		{
			Name:      "run-containerd",
			MountPath: "/host/run/containerd",
		},
		{
			Name:      "run-crio",
			MountPath: "/host/run/crio",
		},
	}
	if specIn.Config.Name != "" {
		specOut.DsSpec.MainContainer.VolumeMounts = append(specOut.DsSpec.MainContainer.VolumeMounts, v1.VolumeMount{
			Name:      "metrics-config-volume",
			MountPath: "/etc/metrics/",
		})
	}
	specOut.DsSpec.Volumes = []v1.Volume{
		{
			Name: "dev-volume",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/dev",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "sys-volume",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/sys",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "pod-resources",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/var/lib/kubelet/pod-resources",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "health",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/var/lib/amd-metrics-exporter",
					Type: &healthCreateHostDirectory,
				},
			},
		},
		{
			Name: "slurm",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/var/run/exporter",
					Type: &healthCreateHostDirectory,
				},
			},
		},
	}
	nonSimVolumes := []v1.Volume{
		{
			Name: "proc",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/proc",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "run-containerd",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/run/containerd",
					Type: &hostPathDirectoryOrCreate,
				},
			},
		},
		{
			Name: "run-crio",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/run/crio",
					Type: &hostPathDirectoryOrCreate,
				},
			},
		},
	}
	if !simEnabled {
		specOut.DsSpec.MainContainer.VolumeMounts = append(specOut.DsSpec.MainContainer.VolumeMounts, nonSimMounts...)
		specOut.DsSpec.Volumes = append(specOut.DsSpec.Volumes, nonSimVolumes...)
	}
	if specIn.Config.Name != "" {
		specOut.DsSpec.Volumes = append(specOut.DsSpec.Volumes, v1.Volume{
			Name: "metrics-config-volume",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: specIn.Config.Name,
					},
				},
			},
		})
	}
	if specOut.RbacConfig.Secret != nil {
		specOut.DsSpec.Volumes = append(specOut.DsSpec.Volumes, v1.Volume{
			Name: "tls-certs",
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: specOut.RbacConfig.Secret.Name,
				},
			},
		})
	}

	return &specOut
}
