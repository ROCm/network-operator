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

package workermgr

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	amdv1alpha1 "github.com/ROCm/network-operator/api/v1alpha1"
	utils "github.com/ROCm/network-operator/internal"
)

const (
	workerContainerName = "worker"
)

var (
	//go:embed scripts/workScript.sh
	workScript string
	//go:embed scripts/undoScript.sh
	undoScript string
)

//go:generate mockgen -source=workermgr.go -package=workermgr -destination=mock_workermgr.go WorkerMgrAPI
type WorkerMgrAPI interface {
	// Work executes the work on given node via worker pod
	Work(ctx context.Context, networkConfig *amdv1alpha1.NetworkConfig, node *v1.Node) error
	// Undo undo the work on given node
	Undo(ctx context.Context, networkConfig *amdv1alpha1.NetworkConfig, node *v1.Node) error
	// GetWorkerPod fetches the worker pod info from cluster
	GetWorkerPod(ctx context.Context, networkConfig *amdv1alpha1.NetworkConfig, node *v1.Node) (*v1.Pod, error)
	// Add a node label to mark that the work is completed
	AddWorkReadyLabel(ctx context.Context, logger logr.Logger, nsn types.NamespacedName, pod *v1.Pod)
	// GetWorkReadyLabel get the label key to mark that the work is completed
	GetWorkReadyLabel(nsn types.NamespacedName) string
	// Remove the node label that indicates the work is completed
	RemoveWorkReadyLabel(ctx context.Context, logger logr.Logger, nsn types.NamespacedName, pod *v1.Pod)
}

type workerMgr struct {
	client client.Client
	scheme *runtime.Scheme
}

// NewWorkerMgr creates a new worker manager
func NewWorkerMgr(client client.Client, scheme *runtime.Scheme) WorkerMgrAPI {
	processor := &workerMgr{
		client: client,
		scheme: scheme,
	}
	return processor
}

// Work executes the work on given node
func (w *workerMgr) Work(ctx context.Context, networkConfig *amdv1alpha1.NetworkConfig, node *v1.Node) error {
	logger := log.FromContext(ctx)
	worker := w.getPodDef(networkConfig, node.Name, utils.WorkAction)
	opRes, err := controllerutil.CreateOrPatch(ctx, w.client, worker, func() error {
		return controllerutil.SetControllerReference(networkConfig, worker, w.scheme)
	})
	if err == nil {
		logger.Info("Reconciled worker",
			"name", worker.Name, "action", utils.WorkAction, "result", opRes)
	}
	return err
}

// Undo undo the work on given node
func (w *workerMgr) Undo(ctx context.Context, networkConfig *amdv1alpha1.NetworkConfig, node *v1.Node) error {
	logger := log.FromContext(ctx)
	undoWorker := w.getPodDef(networkConfig, node.Name, utils.UndoAction)
	opRes, err := controllerutil.CreateOrPatch(ctx, w.client, undoWorker, func() error {
		return controllerutil.SetControllerReference(networkConfig, undoWorker, w.scheme)
	})
	if err == nil {
		logger.Info("Reconciled cleaner",
			"name", undoWorker.Name, "action", utils.UndoAction, "result", opRes)
	}
	return err
}

func (w *workerMgr) AddWorkReadyLabel(ctx context.Context, logger logr.Logger, nsn types.NamespacedName, pod *v1.Pod) {
	node := v1.Node{}
	err := w.client.Get(ctx, types.NamespacedName{Name: pod.Spec.NodeName}, &node)
	if err != nil {
		logger.Error(err, fmt.Sprintf("failed to get node resource %+v", pod.Spec.NodeName))
		return
	}
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]string{
				w.GetWorkReadyLabel(nsn): node.Status.NodeInfo.BootID,
			},
		},
	}
	w.patchNode(ctx, patch, &node, logger)
}

func (w *workerMgr) GetWorkReadyLabel(nsn types.NamespacedName) string {
	return fmt.Sprintf(utils.WorkReadyLabelTemplate, nsn.Namespace, nsn.Name)
}

func (w *workerMgr) RemoveWorkReadyLabel(ctx context.Context, logger logr.Logger, nsn types.NamespacedName, pod *v1.Pod) {
	node := v1.Node{}
	err := w.client.Get(ctx, types.NamespacedName{Name: pod.Spec.NodeName}, &node)
	if err != nil {
		logger.Error(err, fmt.Sprintf("failed to get node resource %+v", pod.Spec.NodeName))
		return
	}
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				w.GetWorkReadyLabel(nsn): nil,
			},
		},
	}
	w.patchNode(ctx, patch, &node, logger)
}

func (w *workerMgr) patchNode(ctx context.Context, patch map[string]interface{}, node *v1.Node, logger logr.Logger) {
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		logger.Error(err, fmt.Sprintf("Failed to marshal node label patch: %+v", err))
		return
	}
	rawPatch := client.RawPatch(types.StrategicMergePatchType, patchBytes)
	if err := w.client.Patch(ctx, node, rawPatch); err != nil {
		logger.Error(err, fmt.Sprintf("Failed to patch node label: %+v", err))
		return
	}
}

func (w *workerMgr) GetWorkerPod(ctx context.Context, networkConfig *amdv1alpha1.NetworkConfig, node *v1.Node) (*v1.Pod, error) {
	// get the existing post process worker pod
	// based on the pod status, determine to do proper action
	pod := &v1.Pod{}
	err := w.client.Get(ctx, types.NamespacedName{
		Namespace: networkConfig.Namespace,
		Name:      utils.GetUpgradeWorkerPodName(networkConfig, node.Name),
	}, pod)
	if err != nil {
		return nil, err
	}
	return pod, nil
}

// getPodSpec generate the pod definition for worker
func (w *workerMgr) getPodDef(networkConfig *amdv1alpha1.NetworkConfig, nodeName, action string) *v1.Pod {
	// pod name
	podName := utils.GetUpgradeWorkerPodName(networkConfig, nodeName)
	// worker image
	utilsContainerImage := utils.DefaultUtilsImage
	if networkConfig.Spec.CommonConfig.UtilsContainer.Image != "" {
		utilsContainerImage = networkConfig.Spec.CommonConfig.UtilsContainer.Image
	}
	// container command
	var command []string
	switch action {
	case utils.WorkAction:
		command = []string{"/bin/bash", "-c", workScript}
	case utils.UndoAction:
		command = []string{"/bin/bash", "-c", undoScript}
	}

	// mount necessary folders
	hostPathDirectory := v1.HostPathDirectory
	volumes := []v1.Volume{
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
			Name: "lib",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/lib/modules",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "dev",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/dev",
					Type: &hostPathDirectory,
				},
			},
		},
	}
	volumeMounts := []v1.VolumeMount{
		{
			Name:      "lib",
			MountPath: "/lib/modules",
		},
		{
			Name:      "sys",
			MountPath: "/sys",
		},
		{
			Name:      "dev",
			MountPath: "/dev",
		},
	}

	worker := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: networkConfig.Namespace,
			Labels: map[string]string{
				utils.WorkerActionLabelKey: action,
			},
		},
		Spec: v1.PodSpec{
			NodeName: nodeName,
			Containers: []v1.Container{
				{
					Name:    workerContainerName,
					Image:   utilsContainerImage,
					Command: command,
					SecurityContext: &v1.SecurityContext{
						RunAsUser:  ptr.To(int64(0)),
						Privileged: ptr.To(true),
					},
					VolumeMounts: volumeMounts,
				},
			},
			// make sure when the amdgpu needs to be unloaded
			// the unload pod is always restarted until the amdgpu is unloaded
			// even if the worker node somehow rebooted during the unload
			// after reboot, the unload pod will be restarted again
			RestartPolicy: v1.RestartPolicyOnFailure,
			Volumes:       volumes,
		},
	}

	// add image pull policy if specified
	if networkConfig.Spec.CommonConfig.UtilsContainer.ImagePullPolicy != "" {
		worker.Spec.Containers[0].ImagePullPolicy = v1.PullPolicy(networkConfig.Spec.CommonConfig.UtilsContainer.ImagePullPolicy)
	}
	// add image pull secret if specified
	if networkConfig.Spec.CommonConfig.UtilsContainer.ImageRegistrySecret != nil {
		worker.Spec.ImagePullSecrets = []v1.LocalObjectReference{
			*networkConfig.Spec.CommonConfig.UtilsContainer.ImageRegistrySecret,
		}
	}

	return worker
}
