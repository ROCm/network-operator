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

package deviceplugin

import (
	_ "embed"
	"fmt"

	protos "github.com/ROCm/common-infra-operator/pkg/protos"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DevicePluginName = "device-plugin"
)

//go:generate mockgen -source=deviceplugin.go -package=deviceplugin -destination=mock_deviceplugin.go DevicePluginAPI
type DevicePluginAPI interface {
	SetDevicePluginAsDesired(ds *appsv1.DaemonSet, dpSpec *protos.DevicePluginSpec) (*runtime.Scheme, error)
}

type devicePlugin struct {
	client      client.Client
	scheme      *runtime.Scheme
	isOpenShift bool
}

func NewDevicePlugin(client client.Client, scheme *runtime.Scheme, isOpenShift bool) DevicePluginAPI {
	return &devicePlugin{
		client:      client,
		scheme:      scheme,
		isOpenShift: isOpenShift,
	}
}

func (dp *devicePlugin) SetDevicePluginAsDesired(ds *appsv1.DaemonSet, dpSpec *protos.DevicePluginSpec) (*runtime.Scheme, error) {
	if ds == nil {
		return dp.scheme, fmt.Errorf("daemon set is not initialized, zero pointer")
	}

	nodeSelector := map[string]string{}
	for key, val := range dpSpec.Selector {
		nodeSelector[key] = val
	}

	imagePullSecrets := []v1.LocalObjectReference{}
	if dpSpec.MainContainer.ImageRegistrySecret != nil {
		imagePullSecrets = append(imagePullSecrets, *dpSpec.MainContainer.ImageRegistrySecret)
	}

	var devicePluginLabelPair = []string{"app.kubernetes.io/name", DevicePluginName}
	matchLabels := map[string]string{
		"daemonset-name":         dpSpec.Name,
		devicePluginLabelPair[0]: devicePluginLabelPair[1],
	}
	// add operator defined labels to match labels
	for key, val := range dpSpec.Labels {
		matchLabels[key] = val
	}

	initContainerImage := dpSpec.InitContainers[0].Image
	if initContainerImage == "" {
		initContainerImage = dpSpec.InitContainers[0].DefaultImage
	}
	mainContainerImage := dpSpec.MainContainer.Image
	if mainContainerImage == "" {
		if dp.isOpenShift {
			mainContainerImage = dpSpec.MainContainer.DefaultUbiImage
		} else {
			mainContainerImage = dpSpec.MainContainer.DefaultImage
		}
	}
	ds.Spec = appsv1.DaemonSetSpec{
		Selector: &metav1.LabelSelector{MatchLabels: matchLabels},
		Template: v1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: matchLabels,
			},
			Spec: v1.PodSpec{
				InitContainers: []v1.Container{
					{
						Name:            "driver-init",
						Image:           initContainerImage,
						Command:         dpSpec.InitContainers[0].Command,
						SecurityContext: &v1.SecurityContext{Privileged: ptr.To(dpSpec.InitContainers[0].IsPrivileged)},
						VolumeMounts:    dpSpec.InitContainers[0].VolumeMounts,
					},
				},
				Containers: []v1.Container{
					{

						Env:             dpSpec.MainContainer.Envs,
						Name:            "device-plugin",
						WorkingDir:      "/root",
						Command:         dpSpec.MainContainer.Command,
						Image:           mainContainerImage,
						SecurityContext: &v1.SecurityContext{Privileged: ptr.To(dpSpec.MainContainer.IsPrivileged)},
						VolumeMounts:    dpSpec.MainContainer.VolumeMounts,
					},
				},
				ImagePullSecrets:   imagePullSecrets,
				PriorityClassName:  "system-node-critical",
				NodeSelector:       nodeSelector,
				ServiceAccountName: dpSpec.ServiceAccountName,
				HostNetwork:        dpSpec.MainContainer.IsHostNetwork,
				Volumes:            dpSpec.Volumes,
			},
		},
	}
	if dpSpec.UpgradePolicy != nil {
		up := dpSpec.UpgradePolicy
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
	if dpSpec.MainContainer.ImagePullPolicy != "" {
		ds.Spec.Template.Spec.Containers[0].ImagePullPolicy = v1.PullPolicy(dpSpec.MainContainer.ImagePullPolicy)
	}
	if len(dpSpec.Tolerations) > 0 {
		ds.Spec.Template.Spec.Tolerations = dpSpec.Tolerations
	} else {
		ds.Spec.Template.Spec.Tolerations = nil
	}
	return dp.scheme, nil
}
