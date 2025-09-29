/*
Copyright 2024 Advanced Micro Devices, Inc.

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

package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	networkv1 "github.com/ROCm/network-operator/api/v1"
)

// AINICReconciler reconciles a AINIC object
type AINICReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=network.amd.com,resources=ainics,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=network.amd.com,resources=ainics/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=network.amd.com,resources=ainics/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *AINICReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the AINIC instance
	ainic := &networkv1.AINIC{}
	err := r.Get(ctx, req.NamespacedName, ainic)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			logger.Info("AINIC resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		logger.Error(err, "Failed to get AINIC")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if ainic.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, ainic)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(ainic, "network.amd.com/finalizer") {
		controllerutil.AddFinalizer(ainic, "network.amd.com/finalizer")
		return ctrl.Result{}, r.Update(ctx, ainic)
	}

	// Reconcile the DaemonSet
	result, err := r.reconcileDaemonSet(ctx, ainic)
	if err != nil {
		logger.Error(err, "Failed to reconcile DaemonSet")
		r.updateStatus(ctx, ainic, "Failed", fmt.Sprintf("Failed to reconcile DaemonSet: %v", err))
		return result, err
	}

	// Update status
	err = r.updateAINICStatus(ctx, ainic)
	if err != nil {
		logger.Error(err, "Failed to update AINIC status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
}

// reconcileDaemonSet creates or updates the DaemonSet for AINIC driver
func (r *AINICReconciler) reconcileDaemonSet(ctx context.Context, ainic *networkv1.AINIC) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-ainic-driver", ainic.Name),
			Namespace: ainic.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, daemonSet, func() error {
		// Set AINIC as the owner of the DaemonSet
		if err := controllerutil.SetControllerReference(ainic, daemonSet, r.Scheme); err != nil {
			return err
		}

		// Configure DaemonSet spec
		daemonSet.Spec = appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": fmt.Sprintf("%s-ainic-driver", ainic.Name),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": fmt.Sprintf("%s-ainic-driver", ainic.Name),
					},
				},
				Spec: r.buildPodSpec(ainic),
			},
		}

		// Apply node selector if specified
		if ainic.Spec.NodeSelector != nil {
			daemonSet.Spec.Template.Spec.NodeSelector = ainic.Spec.NodeSelector
		}

		return nil
	})

	if err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Reconciled DaemonSet", "operation", op, "name", daemonSet.Name)
	return ctrl.Result{}, nil
}

// buildPodSpec creates the pod specification for the AINIC driver
func (r *AINICReconciler) buildPodSpec(ainic *networkv1.AINIC) corev1.PodSpec {
	privileged := true
	hostNetwork := true

	// Build environment variables
	env := []corev1.EnvVar{
		{
			Name: "NODE_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "spec.nodeName",
				},
			},
		},
	}

	// Add custom environment variables
	for _, envVar := range ainic.Spec.Driver.Env {
		env = append(env, corev1.EnvVar{
			Name:  envVar.Name,
			Value: envVar.Value,
		})
	}

	return corev1.PodSpec{
		ServiceAccountName: "ainic-driver",
		HostNetwork:        hostNetwork,
		HostPID:            true,
		Containers: []corev1.Container{
			{
				Name:  "ainic-driver",
				Image: ainic.Spec.Driver.Image,
				Args:  ainic.Spec.Driver.Args,
				Env:   env,
				SecurityContext: &corev1.SecurityContext{
					Privileged: &privileged,
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "host-sys",
						MountPath: "/host/sys",
						ReadOnly:  false,
					},
					{
						Name:      "host-dev",
						MountPath: "/host/dev",
						ReadOnly:  false,
					},
					{
						Name:      "host-proc",
						MountPath: "/host/proc",
						ReadOnly:  true,
					},
				},
			},
		},
		Volumes: []corev1.Volume{
			{
				Name: "host-sys",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/sys",
					},
				},
			},
			{
				Name: "host-dev",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/dev",
					},
				},
			},
			{
				Name: "host-proc",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/proc",
					},
				},
			},
		},
		Tolerations: []corev1.Toleration{
			{
				Operator: corev1.TolerationOpExists,
			},
		},
	}
}

// updateAINICStatus updates the status of the AINIC resource
func (r *AINICReconciler) updateAINICStatus(ctx context.Context, ainic *networkv1.AINIC) error {
	// Get the DaemonSet
	daemonSet := &appsv1.DaemonSet{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      fmt.Sprintf("%s-ainic-driver", ainic.Name),
		Namespace: ainic.Namespace,
	}, daemonSet)
	if err != nil {
		return err
	}

	// Update status based on DaemonSet status
	ainic.Status.NodesTotal = daemonSet.Status.DesiredNumberScheduled
	ainic.Status.NodesReady = daemonSet.Status.NumberReady

	if daemonSet.Status.NumberReady == daemonSet.Status.DesiredNumberScheduled && daemonSet.Status.DesiredNumberScheduled > 0 {
		ainic.Status.Phase = "Ready"
		ainic.Status.Message = "All nodes are ready with AINIC driver"
	} else if daemonSet.Status.NumberReady == 0 {
		ainic.Status.Phase = "Pending"
		ainic.Status.Message = "Waiting for AINIC driver to be scheduled"
	} else {
		ainic.Status.Phase = "Progressing"
		ainic.Status.Message = fmt.Sprintf("AINIC driver ready on %d/%d nodes",
			daemonSet.Status.NumberReady, daemonSet.Status.DesiredNumberScheduled)
	}

	// Update conditions
	r.updateConditions(ainic)

	return r.Status().Update(ctx, ainic)
}

// updateConditions updates the conditions in AINIC status
func (r *AINICReconciler) updateConditions(ainic *networkv1.AINIC) {
	now := metav1.Now()

	// Ready condition
	readyCondition := networkv1.AINICCondition{
		Type:               "Ready",
		LastTransitionTime: now,
	}

	if ainic.Status.Phase == "Ready" {
		readyCondition.Status = metav1.ConditionTrue
		readyCondition.Reason = "AllNodesReady"
		readyCondition.Message = "All nodes have AINIC driver ready"
	} else {
		readyCondition.Status = metav1.ConditionFalse
		readyCondition.Reason = "NotAllNodesReady"
		readyCondition.Message = fmt.Sprintf("AINIC driver ready on %d/%d nodes",
			ainic.Status.NodesReady, ainic.Status.NodesTotal)
	}

	// Update or add the condition
	ainic.Status.Conditions = r.setCondition(ainic.Status.Conditions, readyCondition)
}

// setCondition updates or adds a condition to the conditions slice
func (r *AINICReconciler) setCondition(conditions []networkv1.AINICCondition, newCondition networkv1.AINICCondition) []networkv1.AINICCondition {
	for i, condition := range conditions {
		if condition.Type == newCondition.Type {
			conditions[i] = newCondition
			return conditions
		}
	}
	return append(conditions, newCondition)
}

// updateStatus is a helper to update the status with phase and message
func (r *AINICReconciler) updateStatus(ctx context.Context, ainic *networkv1.AINIC, phase, message string) {
	ainic.Status.Phase = phase
	ainic.Status.Message = message
	r.Status().Update(ctx, ainic)
}

// handleDeletion handles the deletion of AINIC resource
func (r *AINICReconciler) handleDeletion(ctx context.Context, ainic *networkv1.AINIC) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling AINIC deletion", "name", ainic.Name)

	// Remove finalizer
	controllerutil.RemoveFinalizer(ainic, "network.amd.com/finalizer")
	return ctrl.Result{}, r.Update(ctx, ainic)
}

// SetupWithManager sets up the controller with the Manager.
func (r *AINICReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkv1.AINIC{}).
		Owns(&appsv1.DaemonSet{}).
		Complete(r)
}
