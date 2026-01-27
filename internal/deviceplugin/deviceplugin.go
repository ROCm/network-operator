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

package deviceplugininternal

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
	defaultInitContainerImage = "busybox:1.36"
	defaultDevicePluginImage  = "docker.io/rocm/k8s-network-device-plugin:v1.0.0"
	devicePluginSAName        = "amd-network-operator-device-plugin"
	DevicePluginName          = "device-plugin"
)

func GenerateCommonDevicePluginSpec(nwConfig *amdv1alpha1.NetworkConfig) *protos.DevicePluginSpec {
	var dpOut protos.DevicePluginSpec
	specIn := &nwConfig.Spec.DevicePlugin
	simEnabled, _ := strconv.ParseBool(os.Getenv("SIM_ENABLE"))

	dpOut.Name = fmt.Sprintf("%s-%s", nwConfig.Name, DevicePluginName)
	dpOut.Namespace = nwConfig.Namespace
	dpOut.Enable = nwConfig.Spec.Driver.Enable
	dpOut.ServiceAccountName = devicePluginSAName
	dpOut.Tolerations = specIn.DevicePluginTolerations
	dpOut.UpgradePolicy = (*protos.DaemonSetUpgradeSpec)(specIn.UpgradePolicy.DeepCopy())
	dpOut.Selector = nwConfig.Spec.Selector
	dpOut.Labels = map[string]string{
		utils.CRNameLabel: nwConfig.Name,
	}

	initContainer := protos.InitContainerSpec{
		CommonContainerSpec: protos.CommonContainerSpec{
			DefaultImage: defaultInitContainerImage,
			Image:        nwConfig.Spec.CommonConfig.InitContainerImage,
			IsPrivileged: true,
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "sys",
					MountPath: "/sys",
				},
				{
					Name:      "cni",
					MountPath: "/host/etc/cni/",
				},
			},
		},
	}
	if !simEnabled {
		initContainer.Command = []string{
			"sh", "-c",
			`while [ ! -d /sys/class/infiniband ] ||
				[ ! -d /sys/class/infiniband_verbs ] ||
				[ ! -d /sys/module/ionic/drivers ] ||
				! ls /host/etc/cni/net.d/*multus*.conf >/dev/null 2>&1; do
					echo "Waiting for AMD ionic driver and Multus CNI config to be ready"
					sleep 2
			done`,
		}
	} else {
		initContainer.Command = []string{
			"sh", "-c",
			`while [ ! -f /host/etc/cni/net.d/*multus*.conf ]; do
				echo "Waiting for Multus CNI config to be present in /etc/cni/net.d"
				sleep 2
			done`,
		}
	}

	dpOut.InitContainers = []protos.InitContainerSpec{initContainer}
	dpOut.MainContainer.DefaultImage = defaultDevicePluginImage
	dpOut.MainContainer.Image = specIn.DevicePluginImage
	dpOut.MainContainer.ImagePullPolicy = specIn.DevicePluginImagePullPolicy
	dpOut.MainContainer.ImageRegistrySecret = specIn.ImageRegistrySecret
	dpOut.MainContainer.IsPrivileged = true
	dpOut.MainContainer.IsHostNetwork = true
	dpOut.MainContainer.Command = []string{}

	var commandArgs string
	for key, val := range specIn.DevicePluginArguments {
		commandArgs += " -" + key + "=" + val
	}
	//dpOut.MainContainer.Command = []string{"sh", "-c", commandArgs}

	hostPathDirectory := v1.HostPathDirectory
	hostPathDirectoryOrCreate := v1.HostPathDirectoryOrCreate
	dpOut.MainContainer.Envs = []v1.EnvVar{
		{
			Name: "DS_NODE_NAME",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "spec.nodeName",
				},
			},
		},
	}
	dpOut.MainContainer.VolumeMounts = []v1.VolumeMount{
		{
			Name:      "kubelet-device-plugins",
			MountPath: "/var/lib/kubelet/device-plugins",
			ReadOnly:  false,
		},
		{
			Name:      "kubelet-plugins-registry",
			MountPath: "/var/lib/kubelet/plugins_registry",
			ReadOnly:  false,
		},
		{
			Name:      "var-log",
			MountPath: "/var/log",
		},
		{
			Name:      "device-info",
			MountPath: "/var/run/k8s.cni.cncf.io/devinfo/dp",
		},
		{
			Name:      "device-plugin-config-volume",
			MountPath: "/etc/pcidp",
		},
		{
			Name:      "health",
			MountPath: "/var/lib/amd-metrics-exporter",
		},
	}
	dpOut.Volumes = []v1.Volume{
		{
			Name: "kubelet-device-plugins",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/var/lib/kubelet/device-plugins",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "kubelet-plugins-registry",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/var/lib/kubelet/plugins_registry",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "var-log",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/var/log",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "device-info",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/var/run/k8s.cni.cncf.io/devinfo/dp",
					Type: &hostPathDirectoryOrCreate,
				},
			},
		},
		{
			Name: "device-plugin-config-volume",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: os.Getenv("DEVICE_PLUGIN_CONFIG_MAP_NAME"),
					},
				},
			},
		},
		{
			Name: "health",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/var/lib/amd-metrics-exporter",
					Type: &hostPathDirectoryOrCreate,
				},
			},
		},
		{
			Name: "sys",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/sys",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "cni",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/etc/cni",
					Type: &hostPathDirectory,
				},
			},
		},
	}
	return &dpOut
}
