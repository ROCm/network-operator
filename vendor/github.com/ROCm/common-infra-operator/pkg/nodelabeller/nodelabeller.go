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

package nodelabeller

import (
	"fmt"

	protos "github.com/ROCm/common-infra-operator/pkg/protos"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

const (
	NodeLabellerName = "node-labeller"
)

//go:generate mockgen -source=nodelabeller.go -package=nodelabeller -destination=mock_nodelabeller.go NodeLabeller
type NodeLabeller interface {
	SetNodeLabellerAsDesired(ds *appsv1.DaemonSet, nlConfig *protos.NodeLabellerSpec) (*runtime.Scheme, error)
}

type nodeLabeller struct {
	scheme      *runtime.Scheme
	isOpenShift bool
}

func NewNodeLabeller(scheme *runtime.Scheme, isOpenshift bool) NodeLabeller {
	return &nodeLabeller{
		scheme:      scheme,
		isOpenShift: isOpenshift,
	}
}

func (nl *nodeLabeller) SetNodeLabellerAsDesired(ds *appsv1.DaemonSet, nlSpec *protos.NodeLabellerSpec) (*runtime.Scheme, error) {
	if ds == nil {
		return nl.scheme, fmt.Errorf("daemon set is not initialized, zero pointer")
	}
	initContainerImage := nlSpec.InitContainers[0].Image
	if initContainerImage == "" {
		initContainerImage = nlSpec.InitContainers[0].DefaultImage
	}

	mainContainerImage := nlSpec.MainContainer.Image
	if mainContainerImage == "" {
		if nl.isOpenShift {
			mainContainerImage = nlSpec.MainContainer.DefaultUbiImage
		} else {
			mainContainerImage = nlSpec.MainContainer.DefaultImage
		}
	}
	initContainers := []v1.Container{
		{
			Name:            "driver-init",
			Image:           initContainerImage,
			Command:         nlSpec.InitContainers[0].Command,
			SecurityContext: &v1.SecurityContext{Privileged: ptr.To(nlSpec.InitContainers[0].IsPrivileged)},
			VolumeMounts:    nlSpec.InitContainers[0].VolumeMounts,
		},
	}

	imagePullSecrets := []v1.LocalObjectReference{}
	if nlSpec.MainContainer.ImageRegistrySecret != nil {
		imagePullSecrets = append(imagePullSecrets, *nlSpec.MainContainer.ImageRegistrySecret)
	}
	var nodeLabellerLabelPair = []string{"app.kubernetes.io/name", NodeLabellerName}
	matchLabels := map[string]string{
		"daemonset-name":         nlSpec.Name,
		nodeLabellerLabelPair[0]: nodeLabellerLabelPair[1],
	}

	ds.Spec = appsv1.DaemonSetSpec{
		Selector: &metav1.LabelSelector{MatchLabels: matchLabels},
		Template: v1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: matchLabels,
			},
			Spec: v1.PodSpec{
				InitContainers: initContainers,
				Containers: []v1.Container{
					{
						Args:            nlSpec.MainContainer.Arguments,
						Command:         nlSpec.MainContainer.Command,
						Env:             nlSpec.MainContainer.Envs,
						Name:            "node-labeller-container",
						WorkingDir:      "/root",
						Image:           mainContainerImage,
						SecurityContext: &v1.SecurityContext{Privileged: ptr.To(true)},
						VolumeMounts:    nlSpec.MainContainer.VolumeMounts,
					},
				},
				PriorityClassName:  "system-node-critical",
				NodeSelector:       nlSpec.Selector,
				ServiceAccountName: nlSpec.ServiceAccountName,
				Volumes:            nlSpec.Volumes,
				ImagePullSecrets:   imagePullSecrets,
			},
		},
	}

	if nlSpec.UpgradePolicy != nil {
		up := nlSpec.UpgradePolicy
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
	if nlSpec.MainContainer.ImagePullPolicy != "" {
		ds.Spec.Template.Spec.Containers[0].ImagePullPolicy = v1.PullPolicy(nlSpec.MainContainer.ImagePullPolicy)
	}

	if len(nlSpec.Tolerations) > 0 {
		ds.Spec.Template.Spec.Tolerations = nlSpec.Tolerations
	} else {
		ds.Spec.Template.Spec.Tolerations = nil
	}

	return nl.scheme, nil

}
