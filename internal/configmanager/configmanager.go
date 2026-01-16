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

package configmanagerinternal

import (
	"os"
	"strconv"

	protos "github.com/ROCm/common-infra-operator/pkg/protos"
	amdv1alpha1 "github.com/ROCm/network-operator/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
)

const (
	defaultConfigManagerImage = "docker.io/rocm/device-config-manager:latest"
	defaultInitContainerImage = "busybox:1.36"
	dcmSAName                 = "amd-network-operator-config-manager"
)

func GenerateCommonConfigManagerSpec(nwConfig *amdv1alpha1.NetworkConfig) *protos.ConfigManagerSpec {
	var dcmOut protos.ConfigManagerSpec
	specIn := &nwConfig.Spec.ConfigManager
	simEnabled, _ := strconv.ParseBool(os.Getenv("SIM_ENABLE"))

	dcmOut.Name = nwConfig.Name
	dcmOut.Namespace = nwConfig.Namespace
	dcmOut.Enable = specIn.Enable
	dcmOut.ServiceAccountName = dcmSAName
	dcmOut.Tolerations = specIn.ConfigManagerTolerations
	dcmOut.UpgradePolicy = (*protos.DaemonSetUpgradeSpec)(specIn.UpgradePolicy.DeepCopy())
	dcmOut.Selector = nwConfig.Spec.Selector

	dcmOut.InitContainers = make([]protos.InitContainerSpec, 1)
	dcmOut.InitContainers[0].IsPrivileged = true
	dcmOut.InitContainers[0].DefaultImage = defaultInitContainerImage
	dcmOut.InitContainers[0].Image = nwConfig.Spec.CommonConfig.InitContainerImage

	if simEnabled {
		dcmOut.InitContainers[0].Command = []string{}
	} else {
		dcmOut.InitContainers[0].Command = []string{
			"sh", "-c",
			`while [ ! -d /sys/class/infiniband ] || 
			       [ ! -d /sys/class/infiniband_verbs ] || 
			       [ ! -d /sys/module/ionic/drivers ]; do 
		        echo "amd ionic driver is not loaded " 
			    sleep 2 
			done`,
		}
	}

	dcmOut.InitContainers[0].VolumeMounts = []v1.VolumeMount{
		{
			Name:      "sys-volume",
			MountPath: "/host-sys",
		},
	}

	// We use Ubi based images for both vanilla k8s and openshift.
	dcmOut.MainContainer.DefaultImage = defaultConfigManagerImage
	dcmOut.MainContainer.DefaultUbiImage = defaultConfigManagerImage

	dcmOut.MainContainer.Image = specIn.Image
	dcmOut.MainContainer.ImagePullPolicy = specIn.ImagePullPolicy
	dcmOut.MainContainer.ImageRegistrySecret = specIn.ImageRegistrySecret
	dcmOut.MainContainer.IsPrivileged = true

	hostPathDirectory := v1.HostPathDirectory
	hostPathFile := v1.HostPathFile

	dcmOut.MainContainer.Envs = []v1.EnvVar{
		{
			Name: "DS_NODE_NAME",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "spec.nodeName",
				},
			},
		},
		{
			Name: "POD_NAMESPACE",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		{
			Name: "POD_NAME",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
	}

	dcmOut.InitContainers[0].Envs = []v1.EnvVar{
		{
			Name:  "SIM_ENABLE",
			Value: os.Getenv("SIM_ENABLE"),
		},
	}

	dcmOut.MainContainer.VolumeMounts = []v1.VolumeMount{
		{
			Name:      "dev-volume",
			MountPath: "/dev",
		},
		{
			Name:      "sys-volume",
			MountPath: "/sys",
		},
		{
			Name:      "lib-modules",
			MountPath: "/lib/modules",
		},
	}
	nonSimMounts := []v1.VolumeMount{
		{
			Name:      "nicctl",
			MountPath: "/usr/sbin/nicctl",
		},
		{
			Name:      "opt-amd",
			MountPath: "/opt/amd",
		},
		{
			Name:      "metadata",
			MountPath: "/etc/amd/ainic",
		},
	}

	dcmOut.Volumes = []v1.Volume{
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
			Name: "lib-modules",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/lib/modules",
					Type: &hostPathDirectory,
				},
			},
		},
	}
	// rebase to latest and make changes if necessary
	nonSimVolumes := []v1.Volume{
		{
			Name: "nicctl",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/usr/sbin/nicctl",
					Type: &hostPathFile,
				},
			},
		},
		{
			Name: "opt-amd",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/opt/amd",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "metadata",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/etc/amd/ainic",
					Type: &hostPathDirectory,
				},
			},
		},
	}

	if !simEnabled {
		dcmOut.MainContainer.VolumeMounts = append(dcmOut.MainContainer.VolumeMounts, nonSimMounts...)
		dcmOut.Volumes = append(dcmOut.Volumes, nonSimVolumes...)
	}

	return &dcmOut
}
