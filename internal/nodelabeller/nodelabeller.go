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

package nodelabellerinternal

import (
	"fmt"
	"os"
	"strconv"

	v1 "k8s.io/api/core/v1"

	protos "github.com/ROCm/common-infra-operator/pkg/protos"
	amdv1alpha1 "github.com/ROCm/network-operator/api/v1alpha1"
)

const (
	defaultNodeLabellerUbiImage = "docker.io/rocm/k8s-network-node-labeller:v1.1.0"
	defaultInitContainerImage   = "busybox:1.36"
	defaultBlacklistFileName    = "blacklist-ionic-netop.conf"
	nodeLabellerSAName          = "amd-network-operator-node-labeller"
	NodeLabellerNameSuffix      = "node-labeller"
)

func GenerateCommonNodeLabellerSpec(nwConfig *amdv1alpha1.NetworkConfig, isOpenShift bool) *protos.NodeLabellerSpec {
	var nlOut protos.NodeLabellerSpec
	specIn := &nwConfig.Spec.DevicePlugin
	simEnabled, _ := strconv.ParseBool(os.Getenv("SIM_ENABLE"))

	nlOut.Name = nwConfig.Name
	nlOut.Namespace = nwConfig.Namespace
	nlOut.Enable = specIn.EnableNodeLabeller
	nlOut.ServiceAccountName = nodeLabellerSAName
	nlOut.Tolerations = specIn.NodeLabellerTolerations
	nlOut.UpgradePolicy = (*protos.DaemonSetUpgradeSpec)(specIn.UpgradePolicy.DeepCopy())
	nlOut.Selector = nwConfig.Spec.Selector

	nlOut.InitContainers = make([]protos.InitContainerSpec, 1)
	nlOut.InitContainers[0].IsPrivileged = true
	nlOut.InitContainers[0].DefaultImage = defaultInitContainerImage
	nlOut.InitContainers[0].Image = nwConfig.Spec.CommonConfig.InitContainerImage
	nlOut.InitContainers[0].Command = getInitContainerCommand(simEnabled, isOpenShift, nwConfig.Spec.Driver.Blacklist)

	nlOut.InitContainers[0].VolumeMounts = []v1.VolumeMount{
		{
			Name:      "sys-volume",
			MountPath: "/sys",
		},
		{
			Name:      "etc-volume",
			MountPath: "/host-etc",
		},
	}

	// We use Ubi based images for both vanilla k8s and openshift.
	nlOut.MainContainer.DefaultImage = defaultNodeLabellerUbiImage
	nlOut.MainContainer.DefaultUbiImage = defaultNodeLabellerUbiImage

	nlOut.MainContainer.Image = specIn.NodeLabellerImage
	nlOut.MainContainer.ImagePullPolicy = specIn.NodeLabellerImagePullPolicy
	nlOut.MainContainer.ImageRegistrySecret = specIn.ImageRegistrySecret
	nlOut.MainContainer.IsPrivileged = true

	hostPathDirectory := v1.HostPathDirectory

	nlOut.MainContainer.Envs = []v1.EnvVar{
		{
			Name: "DS_NODE_NAME",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "spec.nodeName",
				},
			},
		},
	}
	simEnvvars := []v1.EnvVar{
		{
			Name:  "SIM_ENABLE",
			Value: "1",
		},
	}
	if simEnabled {
		nlOut.MainContainer.Envs = append(nlOut.MainContainer.Envs, simEnvvars...)
	}

	nlOut.MainContainer.VolumeMounts = []v1.VolumeMount{
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

	nlOut.Volumes = []v1.Volume{
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
		{
			Name: "etc-volume",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/etc",
					Type: &hostPathDirectory,
				},
			},
		},
	}

	return &nlOut
}

func getInitContainerCommand(simEnabled, isOpenShift bool, blacklist *bool) []string {
	shouldBlacklist := blacklist != nil && *blacklist

	if simEnabled {
		return getSimCommand(shouldBlacklist)
	}

	if isOpenShift {
		return getOpenShiftWaitCommand()
	}

	return getNonSimCommand(shouldBlacklist)
}

func getSimCommand(blacklist bool) []string {
	if blacklist {
		return []string{
			"sh", "-c",
			fmt.Sprintf(`echo "blacklist ionic" > /host-etc/modprobe.d/%v`, defaultBlacklistFileName),
		}
	}
	return []string{
		"sh", "-c",
		fmt.Sprintf(`rm -f /host-etc/modprobe.d/%v`, defaultBlacklistFileName),
	}
}

func getOpenShiftWaitCommand() []string {
	return []string{
		"sh", "-c",
		`while [ ! -d /sys/class/infiniband ] || 
		       [ ! -d /sys/class/infiniband_verbs ] || 
		       [ ! -d /sys/module/ionic/drivers ]; do 
	        echo "amd ionic driver is not loaded " 
		    sleep 2 
		done`,
	}
}

func getNonSimCommand(blacklist bool) []string {
	waitScript := `while [ ! -d /sys/class/infiniband ] || 
		[ ! -d /sys/class/infiniband_verbs ] || 
		[ ! -d /sys/module/ionic/drivers ]; do 
          echo "amd ionic driver is not loaded " 
        sleep 2 
        done`

	var modprobeAction string
	if blacklist {
		modprobeAction = fmt.Sprintf(`echo "blacklist ionic" > /host-etc/modprobe.d/%v`, defaultBlacklistFileName)
	} else {
		modprobeAction = fmt.Sprintf(`rm -f /host-etc/modprobe.d/%v`, defaultBlacklistFileName)
	}

	return []string{
		"sh", "-c",
		fmt.Sprintf("%s; %s", modprobeAction, waitScript),
	}
}
