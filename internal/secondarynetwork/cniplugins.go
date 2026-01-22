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

package secondarynetwork

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/ROCm/network-operator/api/v1alpha1"
	utils "github.com/ROCm/network-operator/internal"
)

const (
	defaultCNIPluginsImage = "docker.io/rocm/k8s-cni-plugins:v1.0.0"
	CNIPluginsName         = "cni-plugins"
)

var cniPluginsLabelPair = []string{"app.kubernetes.io/name", CNIPluginsName}

//go:generate mockgen -source=cniplugins.go -destination=mock_cniplugins.go -package=secondarynetwork SecondaryNetwork
type SecondaryNetworkAPI interface {
	SetCNIPluginsAsDesired(nwConfigName string, ds *appsv1.DaemonSet, cniPluginsSpec *v1alpha1.CniPluginsSpec, nodeSelector map[string]string) (*runtime.Scheme, error)
}

type secondaryNetwork struct {
	scheme      *runtime.Scheme
	isOpenshift bool
}

func NewSecondaryNetwork(scheme *runtime.Scheme, isOpenshift bool) SecondaryNetworkAPI {
	return &secondaryNetwork{
		scheme:      scheme,
		isOpenshift: isOpenshift,
	}
}

func (s *secondaryNetwork) SetCNIPluginsAsDesired(nwConfigName string, ds *appsv1.DaemonSet, cniPluginsSpec *v1alpha1.CniPluginsSpec, nodeSelector map[string]string) (*runtime.Scheme, error) {
	cniPluginsImage := defaultCNIPluginsImage
	if cniPluginsSpec.Image != "" {
		cniPluginsImage = cniPluginsSpec.Image
	}

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "cni-bin",           // The name of the volume mount
			MountPath: "/host/opt/cni/bin", // The path where it will be mounted inside the container
		},
	}

	hostPathDirectoryOrCreate := corev1.HostPathDirectoryOrCreate
	volumes := []corev1.Volume{
		{
			Name: "cni-bin", // This name must match the one used in the volumeMount
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/opt/cni/bin",             // The path on the host node
					Type: &hostPathDirectoryOrCreate, // creates the directory if it does not exist
				},
			},
		},
	}

	matchLabels := map[string]string{
		"daemonset-name":       ds.Name,
		cniPluginsLabelPair[0]: cniPluginsLabelPair[1], // in amdnetwork namespace
		utils.CRNameLabel:      nwConfigName,
	}

	containers := []corev1.Container{
		{
			Name:         CNIPluginsName + "-container",
			WorkingDir:   "/root",
			Image:        cniPluginsImage,
			VolumeMounts: volumeMounts,
		},
	}
	if cniPluginsSpec.ImagePullPolicy != "" {
		containers[0].ImagePullPolicy = corev1.PullPolicy(cniPluginsSpec.ImagePullPolicy)
	}

	imagePullSecrets := []corev1.LocalObjectReference{}
	if cniPluginsSpec.ImageRegistrySecret != nil {
		imagePullSecrets = append(imagePullSecrets, *cniPluginsSpec.ImageRegistrySecret)
	}

	gracePeriod := int64(1)
	ds.Spec = appsv1.DaemonSetSpec{
		Selector: &metav1.LabelSelector{MatchLabels: matchLabels},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: matchLabels,
			},
			Spec: corev1.PodSpec{
				Containers:                    containers,
				Volumes:                       volumes,
				ImagePullSecrets:              imagePullSecrets,
				NodeSelector:                  nodeSelector,
				TerminationGracePeriodSeconds: &gracePeriod,
			},
		},
	}
	if cniPluginsSpec.UpgradePolicy != nil {
		up := cniPluginsSpec.UpgradePolicy
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

	if len(cniPluginsSpec.Tolerations) > 0 {
		ds.Spec.Template.Spec.Tolerations = cniPluginsSpec.Tolerations
	} else {
		ds.Spec.Template.Spec.Tolerations = nil
	}

	return s.scheme, nil
}
