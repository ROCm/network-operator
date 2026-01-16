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

package controllers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/ROCm/common-infra-operator/pkg/configmanager"
	"github.com/ROCm/common-infra-operator/pkg/deviceplugin"
	"github.com/ROCm/common-infra-operator/pkg/metricsexporter"
	"github.com/ROCm/common-infra-operator/pkg/nodelabeller"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"

	"github.com/rh-ecosystem-edge/kernel-module-management/pkg/labels"

	amdv1alpha1 "github.com/ROCm/network-operator/api/v1alpha1"
	utils "github.com/ROCm/network-operator/internal"
	"github.com/ROCm/network-operator/internal/conditions"
	cminternal "github.com/ROCm/network-operator/internal/configmanager"
	"github.com/ROCm/network-operator/internal/controllers/watchers"
	dpinternal "github.com/ROCm/network-operator/internal/deviceplugin"
	"github.com/ROCm/network-operator/internal/kmmmodule"
	expinternal "github.com/ROCm/network-operator/internal/metricsexporter"
	nlinternal "github.com/ROCm/network-operator/internal/nodelabeller"
	"github.com/ROCm/network-operator/internal/secondarynetwork"
	"github.com/ROCm/network-operator/internal/validator"
	"github.com/ROCm/network-operator/internal/workermgr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	kmmv1beta1 "github.com/rh-ecosystem-edge/kernel-module-management/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	event "sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	NetworkConfigReconcilerName = "DriverAndPluginReconciler"
	networkConfigFinalizer      = "amd.node.kubernetes.io/networkconfig-finalizer"
)

// ModuleReconciler reconciles a Module object
type NetworkConfigReconciler struct {
	once            sync.Once
	initErr         error
	helper          networkConfigReconcilerHelperAPI
	podEventHandler watchers.PodEventHandlerAPI
	isOpenShift     bool
}

func NewNetworkConfigReconciler(
	k8sConfig *rest.Config,
	client client.Client,
	kmmHandler kmmmodule.KMMModuleAPI,
	nlHandler nodelabeller.NodeLabeller,
	metricsHandler metricsexporter.MetricsExporter,
	devicepluginHandler deviceplugin.DevicePluginAPI,
	secondaryNetworkHandler secondarynetwork.SecondaryNetworkAPI,
	workerMgr workermgr.WorkerMgrAPI,
	configmanagerHandler configmanager.ConfigManager,
	isOpenShift bool) *NetworkConfigReconciler {
	upgradeMgrHandler := newUpgradeMgrHandler(client, k8sConfig, isOpenShift, workerMgr)
	helper := newNetworkConfigReconcilerHelper(client, kmmHandler, nlHandler, upgradeMgrHandler, metricsHandler, devicepluginHandler, secondaryNetworkHandler, workerMgr, configmanagerHandler)
	podEventHandler := watchers.NewPodEventHandler(client, workerMgr)
	return &NetworkConfigReconciler{
		helper:          helper,
		podEventHandler: podEventHandler,
		isOpenShift:     isOpenShift,
	}
}

// SetupWithManager sets up the controller with the Manager.
//  1. Owns() will tell the manager that if any Module or Daemonset object or their status got updated
//     the NetworkConfig object in their ref field need to be reconciled
//  2. findNetworkConfigsForNMC: when a NMC changed, only trigger reconcile for related NetworkConfig
func (r *NetworkConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&amdv1alpha1.NetworkConfig{}).
		Owns(&kmmv1beta1.Module{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&v1.Service{}).
		Named(NetworkConfigReconcilerName).
		Watches( // watch NMC for updating the NetworkConfigs CR status
			&kmmv1beta1.NodeModulesConfig{},
			handler.EnqueueRequestsFromMapFunc(r.helper.findNetworkConfigsForNMC),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Watches(&v1.Secret{}, // watch for KMM build/sign/install related secrets
			handler.EnqueueRequestsFromMapFunc(r.helper.findNetworkConfigsForSecret),
			builder.WithPredicates(
				predicate.Funcs{
					CreateFunc: func(e event.CreateEvent) bool {
						return true
					},
					UpdateFunc: func(e event.UpdateEvent) bool {
						return true
					},
					DeleteFunc: func(e event.DeleteEvent) bool {
						return true
					},
				},
			),
		).
		Watches(&v1.Node{}, // watch for Node resource to get latest kernel mapping for KMM CR
			handler.EnqueueRequestsFromMapFunc(r.helper.findNetworkConfigsWithKMM),
			builder.WithPredicates(NodeKernelVersionPredicate{}),
		).
		Watches( // watch pod event to auto-clean unknown status builder pod and cleanup workermgr pod
			&v1.Pod{},
			r.podEventHandler,
			builder.WithPredicates(watchers.PodLabelPredicate{}),
		).Complete(r)
}

func (r *NetworkConfigReconciler) init(ctx context.Context) {
	// List existing Network Configs
	networkConfigList, err := r.helper.listNetworkConfigs(ctx)
	if err != nil {
		r.initErr = err
		return
	}
	r.initErr = r.helper.buildNodeAssignments(networkConfigList)
}

//+kubebuilder:rbac:groups=amd.com,resources=networkconfigs,verbs=get;list;watch;create;patch;update
//+kubebuilder:rbac:groups=amd.com,resources=networkconfigs/status,verbs=get;patch;update
//+kubebuilder:rbac:groups=amd.com,resources=networkconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups=kmm.sigs.x-k8s.io,resources=modules,verbs=get;list;watch;create;patch;update;delete
//+kubebuilder:rbac:groups=kmm.sigs.x-k8s.io,resources=modules/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kmm.sigs.x-k8s.io,resources=modules/finalizers,verbs=get;update;watch
//+kubebuilder:rbac:groups=kmm.sigs.x-k8s.io,resources=nodemodulesconfigs,verbs=get;list;watch
//+kubebuilder:rbac:groups=kmm.sigs.x-k8s.io,resources=nodemodulesconfigs/status,verbs=get;list;watch
//+kubebuilder:rbac:groups=kmm.sigs.x-k8s.io,resources=nodemodulesconfigs/finalizers,verbs=get;update;watch
//+kubebuilder:rbac:groups=nfd.openshift.io,resources=nodefeaturediscoveries,verbs=list;get;delete
//+kubebuilder:rbac:groups=nfd.openshift.io,resources=nodefeaturediscoveries/status,verbs=get;update
//+kubebuilder:rbac:groups=nfd.openshift.io,resources=nodefeaturediscoveries/finalizers,verbs=get;update
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=create;delete;get;list;patch;watch;create
//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get;patch;list;watch
//+kubebuilder:rbac:groups=core,resources=nodes/status,verbs=get;update;watch
//+kubebuilder:rbac:groups=core,resources=nodes/finalizers,verbs=get;update;watch
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=create;delete;get;list;patch;watch
//+kubebuilder:rbac:groups=apps,resources=daemonsets/status,verbs=create;delete;get;list;patch;watch
//+kubebuilder:rbac:groups=apps,resources=daemonsets/finalizers,verbs=create;get;update;watch
//+kubebuilder:rbac:groups=core,resources=services,verbs=create;delete;get;list;patch;watch
//+kubebuilder:rbac:groups=core,resources=services/finalizers,verbs=create;get;update;watch
//+kubebuilder:rbac:groups=core,resources=pods,verbs=delete;get;list;watch;create
//+kubebuilder:rbac:groups=core,resources=pods/status,verbs=delete;get;list;watch
//+kubebuilder:rbac:groups=core,resources=pods/finalizers,verbs=delete;get;list;watch
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=create;delete;get;list;patch;watch
//+kubebuilder:rbac:groups=core,resources=pods/eviction,verbs=delete;get;list;create
//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;delete
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete

func (r *NetworkConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	res := ctrl.Result{}

	logger := log.FromContext(ctx)

	r.once.Do(func() {
		r.init(ctx)
	})
	if r.initErr != nil {
		return res, r.initErr
	}

	nwConfig, err := r.helper.getRequestedNetworkConfig(ctx, req.NamespacedName)
	if err != nil {
		if k8serrors.IsNotFound(err) || strings.Contains(err.Error(), "not found") {
			logger.Info("NetworkConfig CR deleted")
			r.helper.updateNodeAssignments(req.NamespacedName.String(), nil, true)
			return ctrl.Result{}, nil
		}
		return res, fmt.Errorf("failed to get the requested %s CR: %v", req.NamespacedName, err)
	}

	nodes, err := kmmmodule.GetK8SNodes(kmmmodule.MapToLabelSelector(nwConfig.Spec.Selector))
	if err != nil {
		return res, fmt.Errorf("failed to list Node for NetworkConfig %s: %v", req.NamespacedName, err)
	}

	if nwConfig.GetDeletionTimestamp() != nil {
		// Reset the upgrade states
		if _, err := r.helper.handleModuleUpgrade(ctx, nwConfig, nodes, true); err != nil {
			logger.Error(err, fmt.Sprintf("upgrade manager delete network config error: %v", err))
		}
		// NetworkConfig is being deleted
		err = r.helper.finalizeNetworkConfig(ctx, nwConfig, nodes)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to finalize NetworkConfig %s: %v", req.NamespacedName, err)
		}
		return ctrl.Result{}, nil
	}

	// Verify that the NetworkConfig does not select nodes covered by other NetworkConfigs
	err = r.helper.validateNodeAssignments(req.NamespacedName.String(), nodes)
	if err != nil {
		if errSet := r.helper.setCondition(ctx, conditions.ConditionTypeError, nwConfig, metav1.ConditionTrue, conditions.ValidationError, fmt.Sprintf("Validation failed: %v", err)); errSet != nil {
			logger.Error(fmt.Errorf("Failed to set error condition: %v", errSet), "")
		}
		if errSet := r.helper.setCondition(ctx, conditions.ConditionTypeReady, nwConfig, metav1.ConditionFalse, conditions.ReadyStatus, ""); errSet != nil {
			logger.Error(fmt.Errorf("Failed to set ready condition: %v", errSet), "")
		}
		return res, err
	}

	// Validate network config
	result := r.helper.validateNetworkConfig(ctx, nwConfig)
	if len(result) != 0 {
		// Update status Conditions here
		if errSet := r.helper.setCondition(ctx, conditions.ConditionTypeError, nwConfig, metav1.ConditionTrue, conditions.ValidationError, fmt.Sprintf("Validation failed: %v", result)); errSet != nil {
			logger.Error(fmt.Errorf("Failed to set error condition: %v", errSet), "")
		}
		if errSet := r.helper.setCondition(ctx, conditions.ConditionTypeReady, nwConfig, metav1.ConditionFalse, conditions.ReadyStatus, ""); errSet != nil {
			logger.Error(fmt.Errorf("Failed to set ready condition: %v", errSet), "")
		}
		return res, fmt.Errorf("validation failed for NetworkConfig %s: %v", req.NamespacedName, result)
	}

	err = r.helper.setFinalizer(ctx, nwConfig)
	if err != nil {
		return res, fmt.Errorf("failed to set finalizer for NetworkConfig %s: %v", req.NamespacedName, err)
	}

	logger.Info("start build configmap reconciliation")
	err = r.helper.handleBuildConfigMap(ctx, nwConfig, nodes)
	if err != nil {
		return res, fmt.Errorf("failed to handle build ConfigMap for NetworkConfig %s: %v", req.NamespacedName, err)
	}

	logger.Info("start module install/upgrade reconciliation")
	res, err = r.helper.handleModuleUpgrade(ctx, nwConfig, nodes, false)
	if err != nil {
		return res, fmt.Errorf("Failed to fetch nodes for NetworkConfig %s: %v", req.NamespacedName, err)
	}

	logger.Info("start KMM reconciliation")
	if err = r.helper.handleKMMModule(ctx, nwConfig, nodes); err != nil {
		return res, fmt.Errorf("failed to handle KMM module for NetworkConfig %s: %v", req.NamespacedName, err)
	}

	logger.Info("start device-plugin reconciliation")
	if err = r.helper.handleDevicePlugin(ctx, nwConfig); err != nil {
		return res, fmt.Errorf("failed to handle device-plugin for NetworkConfig %s: %v", req.NamespacedName, err)
	}

	logger.Info("start kmm mod version label reconciliation")
	err = r.helper.handleKMMVersionLabel(ctx, nwConfig, nodes)
	if err != nil {
		return res, fmt.Errorf("failed to handle kmm mod version label for NetworkConfig %s: %v", req.NamespacedName, err)
	}

	logger.Info("start node labeller reconciliation")
	err = r.helper.handleNodeLabeller(ctx, nwConfig, nodes, r.isOpenShift)
	if err != nil {
		return res, fmt.Errorf("failed to handle node labeller for NetworkConfig %s: %v", req.NamespacedName, err)
	}

	logger.Info("start metrics exporter reconciliation", "enable", nwConfig.Spec.MetricsExporter.Enable)
	if err := r.helper.handleMetricsExporter(ctx, nwConfig); err != nil {
		return res, fmt.Errorf("failed to handle metrics exporter for NetworkConfig %s: %v", req.NamespacedName, err)
	}

	logger.Info("start secondary network plugins reconciliation")
	if err := r.helper.handleSecondaryNetwork(ctx, nwConfig); err != nil {
		return res, fmt.Errorf("failed to handle secondary network for NetworkConfig %s: %v", req.NamespacedName, err)
	}
	/*--- To be enabled later
	  	logger.Info("start test runner reconciliation", "enable", nwConfig.Spec.TestRunner.Enable)
	  	if err := r.helper.handleTestRunner(ctx, nwConfig, nodes); err != nil {
	          return res, fmt.Errorf("failed to handle test runner for NetworkConfig %s: %v", req.NamespacedName, err)
	  	}
		---*/
	logger.Info("start config manager reconciliation", "enable", nwConfig.Spec.ConfigManager.Enable)
	if err := r.helper.handleConfigManager(ctx, nwConfig); err != nil {
		return res, fmt.Errorf("failed to handle config manager for NetworkConfig %s: %v", req.NamespacedName, err)
	}

	err = r.helper.buildNetworkConfigStatus(ctx, nwConfig, nodes)
	if err != nil {
		return res, fmt.Errorf("failed to build status for NetworkConfig %s: %v", req.NamespacedName, err)
	}

	err = r.helper.updateNetworkConfigStatus(ctx, nwConfig)
	if err != nil {
		return res, fmt.Errorf("failed to update status for NetworkConfig %s: %v", req.NamespacedName, err)
	}

	// Update nodeAssignments after NetworkConfig status update
	r.helper.updateNodeAssignments(req.NamespacedName.String(), nodes, false)

	return res, nil
}

//go:generate mockgen -source=network_config_reconciler.go -package=controllers -destination=mock_network_config_reconciler.go networkConfigReconcilerHelperAPI
type networkConfigReconcilerHelperAPI interface {
	getRequestedNetworkConfig(ctx context.Context, namespacedName types.NamespacedName) (*amdv1alpha1.NetworkConfig, error)
	listNetworkConfigs(ctx context.Context) (*amdv1alpha1.NetworkConfigList, error)
	buildNodeAssignments(networkConfigList *amdv1alpha1.NetworkConfigList) error
	validateNodeAssignments(namespacedName string, nodes *v1.NodeList) error
	updateNodeAssignments(namespacedName string, nodes *v1.NodeList, isFinalizer bool)
	getNetworkConfigOwnedKMMModule(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig) (*kmmv1beta1.Module, error)
	buildNetworkConfigStatus(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig, nodes *v1.NodeList) error
	updateNetworkConfigStatus(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig) error
	finalizeNetworkConfig(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig, nodes *v1.NodeList) error
	findNetworkConfigsForNMC(ctx context.Context, nmc client.Object) []reconcile.Request
	findNetworkConfigsForSecret(ctx context.Context, secret client.Object) []reconcile.Request
	findNetworkConfigsWithKMM(ctx context.Context, node client.Object) []reconcile.Request
	setFinalizer(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig) error
	handleKMMModule(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig, nodes *v1.NodeList) error
	handleDevicePlugin(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig) error
	handleKMMVersionLabel(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig, nodes *v1.NodeList) error
	handleBuildConfigMap(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig, nodes *v1.NodeList) error
	handleNodeLabeller(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig, nodes *v1.NodeList, isOpenShift bool) error
	handleMetricsExporter(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig) error
	handleConfigManager(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig) error
	handleSecondaryNetwork(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig) error
	setCondition(ctx context.Context, condition string, nwConfig *amdv1alpha1.NetworkConfig, status metav1.ConditionStatus, reason string, message string) error
	deleteCondition(ctx context.Context, condition string, nwConfig *amdv1alpha1.NetworkConfig) error
	validateNetworkConfig(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig) []string
	handleModuleUpgrade(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig, nodes *v1.NodeList, delete bool) (ctrl.Result, error)
}

type networkConfigReconcilerHelper struct {
	client                  client.Client
	kmmHandler              kmmmodule.KMMModuleAPI
	nlHandler               nodelabeller.NodeLabeller
	metricsHandler          metricsexporter.MetricsExporter
	devicepluginHandler     deviceplugin.DevicePluginAPI
	configmanagerHandler    configmanager.ConfigManager
	secondaryNetworkHandler secondarynetwork.SecondaryNetworkAPI
	nodeAssignments         map[string]string
	conditionUpdater        conditions.ConditionUpdater
	validator               validator.ValidatorAPI
	upgradeMgrHandler       upgradeMgrAPI
	workerMgr               workermgr.WorkerMgrAPI
	namespace               string
}

func newNetworkConfigReconcilerHelper(client client.Client,
	kmmHandler kmmmodule.KMMModuleAPI,
	nlHandler nodelabeller.NodeLabeller,
	upgradeMgrHandler upgradeMgrAPI,
	metricsHandler metricsexporter.MetricsExporter,
	devicepluginHandler deviceplugin.DevicePluginAPI,
	secondaryNetworkHandler secondarynetwork.SecondaryNetworkAPI,
	workerMgr workermgr.WorkerMgrAPI,
	configmanagerHandler configmanager.ConfigManager) networkConfigReconcilerHelperAPI {
	conditionUpdater := conditions.NewNetworkConfigConditionMgr()
	validator := validator.NewValidator()
	return &networkConfigReconcilerHelper{
		client:                  client,
		kmmHandler:              kmmHandler,
		nlHandler:               nlHandler,
		metricsHandler:          metricsHandler,
		devicepluginHandler:     devicepluginHandler,
		configmanagerHandler:    configmanagerHandler,
		secondaryNetworkHandler: secondaryNetworkHandler,
		nodeAssignments:         make(map[string]string),
		conditionUpdater:        conditionUpdater,
		validator:               validator,
		upgradeMgrHandler:       upgradeMgrHandler,
		workerMgr:               workerMgr,
		namespace:               os.Getenv("OPERATOR_NAMESPACE"),
	}
}

func (dcrh *networkConfigReconcilerHelper) listNetworkConfigs(ctx context.Context) (*amdv1alpha1.NetworkConfigList, error) {
	nwConfigList := amdv1alpha1.NetworkConfigList{}

	if err := dcrh.client.List(ctx, &nwConfigList); err != nil {
		return nil, fmt.Errorf("failed to list NetworkConfigs: %v", err)
	}

	return &nwConfigList, nil
}

func (dcrh *networkConfigReconcilerHelper) getRequestedNetworkConfig(ctx context.Context, namespacedName types.NamespacedName) (*amdv1alpha1.NetworkConfig, error) {
	nwConfig := amdv1alpha1.NetworkConfig{}

	if err := dcrh.client.Get(ctx, namespacedName, &nwConfig); err != nil {
		return nil, fmt.Errorf("failed to get NetworkConfig %s: %v", namespacedName, err)
	}
	return &nwConfig, nil
}

// findNetworkConfigsForNMC when a NMC changed, only trigger reconcile for related NetworkConfig
func (drch *networkConfigReconcilerHelper) findNetworkConfigsForNMC(ctx context.Context, nmc client.Object) []reconcile.Request {
	reqs := []reconcile.Request{}
	logger := log.FromContext(ctx)
	nmcObj, ok := nmc.(*kmmv1beta1.NodeModulesConfig)
	if !ok {
		logger.Error(fmt.Errorf("failed to convert object %+v to NodeModulesConfig", nmc), "")
		return reqs
	}
	if len(nmcObj.Status.Modules) > 0 {
		for _, module := range nmcObj.Status.Modules {
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: module.Namespace,
					Name:      module.Name,
				},
			})
		}
	}
	return reqs
}

// findNetworkConfigsForSecret when a secret changed, only trigger reconcile for related NetworkConfig
func (drch *networkConfigReconcilerHelper) findNetworkConfigsForSecret(ctx context.Context, secret client.Object) []reconcile.Request {
	reqs := []reconcile.Request{}
	logger := log.FromContext(ctx)
	secretObj, ok := secret.(*v1.Secret)
	if !ok {
		logger.Error(fmt.Errorf("failed to convert object %+v to Secret", secret), "")
		return reqs
	}
	if secretObj.Namespace != drch.namespace {
		return reqs
	}
	networkConfigList, err := drch.listNetworkConfigs(ctx)
	if err != nil || networkConfigList == nil {
		logger.Error(err, "failed to list networkconfigs")
		return reqs
	}
	for _, dcfg := range networkConfigList.Items {
		if dcfg.Namespace == drch.namespace &&
			drch.hasSecretReference(secretObj.Name, dcfg) {
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: dcfg.Namespace,
					Name:      dcfg.Name,
				},
			})
		}
	}

	return reqs
}

func (dcrh *networkConfigReconcilerHelper) hasSecretReference(secretName string, dcfg amdv1alpha1.NetworkConfig) bool {
	// these secrets are KMM driver build/sign/install related secrets
	// wrong configuration of them is hard to debug unless dumping logs
	// when their secrets are corrected up and a secret event kicks in
	// reconcile the corresponding networkconfigs CRs who have references
	if dcfg.Spec.Driver.ImageRegistrySecret != nil && dcfg.Spec.Driver.ImageRegistrySecret.Name == secretName {
		return true
	}
	if dcfg.Spec.Driver.ImageSign.KeySecret != nil && dcfg.Spec.Driver.ImageSign.KeySecret.Name == secretName {
		return true
	}
	if dcfg.Spec.Driver.ImageSign.CertSecret != nil && dcfg.Spec.Driver.ImageSign.CertSecret.Name == secretName {
		return true
	}
	return false
}

// findNetworkConfigsWithKMM only reconcile networkconfigs with KMM enabled to manage out-of-tree kernel module
func (drch *networkConfigReconcilerHelper) findNetworkConfigsWithKMM(ctx context.Context, node client.Object) []reconcile.Request {
	reqs := []reconcile.Request{}
	logger := log.FromContext(ctx)
	networkConfigList, err := drch.listNetworkConfigs(ctx)
	if err != nil || networkConfigList == nil {
		logger.Error(err, "failed to list networkconfigs")
		return reqs
	}
	for _, dcfg := range networkConfigList.Items {
		if dcfg.Namespace == drch.namespace &&
			dcfg.Spec.Driver.Enable != nil &&
			*dcfg.Spec.Driver.Enable {
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: dcfg.Namespace,
					Name:      dcfg.Name,
				},
			})
		}
	}

	return reqs
}

func (dcrh *networkConfigReconcilerHelper) buildNetworkConfigStatus(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig, nodes *v1.NodeList) error {
	// fetch NetworkConfig-owned custom resource
	// then retrieve its status and put it to NetworkConfig's status fields
	if nwConfig.Spec.Driver.Enable != nil && *nwConfig.Spec.Driver.Enable {
		kmmModuleObj, err := dcrh.getNetworkConfigOwnedKMMModule(ctx, nwConfig)
		if err != nil {
			return fmt.Errorf("failed to fetch owned kmm module for NetworkConfig %+v: %+v",
				types.NamespacedName{Namespace: nwConfig.Namespace, Name: nwConfig.Name}, err)
		}
		if kmmModuleObj != nil {
			nwConfig.Status.Drivers = amdv1alpha1.DeploymentStatus{
				NodesMatchingSelectorNumber: kmmModuleObj.Status.ModuleLoader.DesiredNumber,
				DesiredNumber:               kmmModuleObj.Status.ModuleLoader.DesiredNumber,
				AvailableNumber:             kmmModuleObj.Status.ModuleLoader.AvailableNumber,
			}
		}
	}

	devPlDs := appsv1.DaemonSet{}
	dsName := types.NamespacedName{
		Namespace: nwConfig.Namespace,
		Name:      fmt.Sprintf("%s-%s", nwConfig.Name, dpinternal.DevicePluginName),
	}

	if err := dcrh.client.Get(ctx, dsName, &devPlDs); err == nil {
		nwConfig.Status.DevicePlugin = amdv1alpha1.DeploymentStatus{
			NodesMatchingSelectorNumber: devPlDs.Status.NumberAvailable,
			DesiredNumber:               devPlDs.Status.DesiredNumberScheduled,
			AvailableNumber:             devPlDs.Status.NumberAvailable,
		}
	} else {
		return fmt.Errorf("failed to fetch device-plugin %+v: %+v", dsName, err)
	}

	if nwConfig.Spec.MetricsExporter.Enable != nil && *nwConfig.Spec.MetricsExporter.Enable {
		metricsDS := appsv1.DaemonSet{}
		dsName := types.NamespacedName{
			Namespace: nwConfig.Namespace,
			Name:      nwConfig.Name + "-" + metricsexporter.ExporterName,
		}

		if err := dcrh.client.Get(ctx, dsName, &metricsDS); err == nil {
			nwConfig.Status.MetricsExporter = amdv1alpha1.DeploymentStatus{
				NodesMatchingSelectorNumber: metricsDS.Status.NumberAvailable,
				DesiredNumber:               metricsDS.Status.DesiredNumberScheduled,
				AvailableNumber:             metricsDS.Status.NumberAvailable,
			}
		} else {
			return fmt.Errorf("failed to fetch metricsExporter %+v: %+v", dsName, err)
		}
	}

	// fetch latest node modules config, push their status back to NetworkConfig's status fields
	if err := dcrh.updateNetworkConfigNodeStatus(ctx, nwConfig, nodes); err != nil {
		return err
	}

	// Successfully processed the config
	nwConfig.Status.ObservedGeneration = nwConfig.Generation
	dcrh.conditionUpdater.DeleteErrorCondition(nwConfig)
	dcrh.conditionUpdater.SetReadyCondition(nwConfig, metav1.ConditionTrue, conditions.ReadyStatus, "")

	return nil
}

func (dcrh *networkConfigReconcilerHelper) updateNetworkConfigStatus(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig) error {
	// get the latest version of object right before update
	// to avoid issue "the object has been modified; please apply your changes to the latest version and try again"
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latestObj, err := dcrh.getRequestedNetworkConfig(ctx, types.NamespacedName{Namespace: nwConfig.Namespace, Name: nwConfig.Name})
		if err != nil {
			return err
		}
		nwConfig.Status.DeepCopyInto(&latestObj.Status)
		if err := dcrh.client.Status().Update(ctx, latestObj); err != nil {
			return err
		}
		return nil
	})
}

func (dcrh *networkConfigReconcilerHelper) getNetworkConfigOwnedKMMModule(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig) (*kmmv1beta1.Module, error) {
	module := kmmv1beta1.Module{}
	namespacedName := types.NamespacedName{Namespace: nwConfig.Namespace, Name: nwConfig.Name}
	if err := dcrh.client.Get(ctx, namespacedName, &module); err != nil {
		return nil, fmt.Errorf("failed to get KMM Module %s: %v", namespacedName, err)
	}
	return &module, nil
}

func (dcrh *networkConfigReconcilerHelper) updateNetworkConfigNodeStatus(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig, nodes *v1.NodeList) error {
	logger := log.FromContext(ctx)
	previousUpgradeTimes := make(map[string]string)
	previousBootIds := make(map[string]string)
	// Persist the UpgradeStartTime
	for nodeName, moduleStatus := range nwConfig.Status.NodeModuleStatus {
		previousUpgradeTimes[nodeName] = moduleStatus.UpgradeStartTime
		previousBootIds[nodeName] = moduleStatus.BootId
	}
	nwConfig.Status.NodeModuleStatus = map[string]amdv1alpha1.ModuleStatus{}

	// for each node, fetch its status of modules configured by given NetworkConfig
	for _, node := range nodes.Items {
		// if there is no module configured for given node
		// the info under that node name will have only status and upgrade start time
		// then it will be clear to see which node didn't get module configured

		upgradeStartTime := dcrh.upgradeMgrHandler.GetNodeUpgradeStartTime(node.Name)
		//If operator restarted during Upgrade, then fetch previous known upgrade start time since the internal maps would have been cleared
		if upgradeStartTime == "" {
			upgradeStartTime = previousUpgradeTimes[node.Name]
		}
		bootId := dcrh.upgradeMgrHandler.GetNodeBootId(node.Name)
		//If operator restarted during Upgrade, then fetch previous known bootId since the internal maps would have been cleared
		if bootId == "" {
			bootId = previousBootIds[node.Name]
		}
		nwConfig.Status.NodeModuleStatus[node.Name] = amdv1alpha1.ModuleStatus{Status: dcrh.upgradeMgrHandler.GetNodeStatus(node.Name), UpgradeStartTime: upgradeStartTime, BootId: bootId}

		nmc := kmmv1beta1.NodeModulesConfig{}
		err := dcrh.client.Get(ctx, types.NamespacedName{Name: node.Name}, &nmc)
		if err != nil {
			if !k8serrors.IsNotFound(err) {
				logger.Error(err, fmt.Sprintf("failed to fetch NMC for node %+v", node.Name))
			}
			continue
		}
		if nmc.Status.Modules != nil {
			for _, module := range nmc.Status.Modules {
				// if there is any module was configured by given NetworkConfig
				// push their status back to NetworkConfig
				if module.Namespace == nwConfig.Namespace &&
					module.Name == nwConfig.Name {
					nwConfig.Status.NodeModuleStatus[node.Name] = amdv1alpha1.ModuleStatus{
						ContainerImage:     module.Config.ContainerImage,
						KernelVersion:      module.Config.KernelVersion,
						LastTransitionTime: module.LastTransitionTime.String(),
						Status:             dcrh.upgradeMgrHandler.GetNodeStatus(node.Name),
						UpgradeStartTime:   upgradeStartTime,
						BootId:             bootId,
					}
				}
			}
		}
	}

	return nil
}

func (dcrh *networkConfigReconcilerHelper) setFinalizer(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig) error {
	if controllerutil.ContainsFinalizer(nwConfig, networkConfigFinalizer) {
		return nil
	}

	nwConfigCopy := nwConfig.DeepCopy()
	controllerutil.AddFinalizer(nwConfig, networkConfigFinalizer)
	return dcrh.client.Patch(ctx, nwConfig, client.MergeFrom(nwConfigCopy))
}

func (dcrh *networkConfigReconcilerHelper) finalizeMetricsExporter(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig) error {
	logger := log.FromContext(ctx)

	// Handle ServiceMonitor deletion
	serviceMonitor := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: nwConfig.Namespace,
			Name:      fmt.Sprintf("%s-%s", nwConfig.Name, metricsexporter.ExporterName),
		},
	}
	if err := dcrh.client.Get(ctx, client.ObjectKeyFromObject(serviceMonitor), serviceMonitor); err != nil {
		if !k8serrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			return fmt.Errorf("failed to get ServiceMonitor %s: %v", serviceMonitor.Name, err)
		}
	} else {
		logger.Info("deleting ServiceMonitor", "ServiceMonitor", serviceMonitor.Name)
		if err := dcrh.client.Delete(ctx, serviceMonitor); err != nil {
			return fmt.Errorf("failed to delete ServiceMonitor %s: %v", serviceMonitor.Name, err)
		}
	}

	// Handle Service deletion
	metricsSvc := v1.Service{}
	svcName := types.NamespacedName{
		Namespace: nwConfig.Namespace,
		Name:      nwConfig.Name + "-" + metricsexporter.ExporterName,
	}

	if err := dcrh.client.Get(ctx, svcName, &metricsSvc); err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to get metrics exporter service %s: %v", svcName, err)
		}
	} else {
		logger.Info("deleting metrics exporter service", "service", svcName)
		if err := dcrh.client.Delete(ctx, &metricsSvc); err != nil {
			return fmt.Errorf("failed to delete metrics exporter service %s: %v", svcName, err)
		}
	}

	// Handle DaemonSet deletion
	metricsDS := appsv1.DaemonSet{}
	dsName := types.NamespacedName{
		Namespace: nwConfig.Namespace,
		Name:      nwConfig.Name + "-" + metricsexporter.ExporterName,
	}

	if err := dcrh.client.Get(ctx, dsName, &metricsDS); err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to get metrics exporter daemonset %s: %v", dsName, err)
		}
	} else {
		logger.Info("deleting metrics exporter daemonset", "daemonset", dsName)
		if err := dcrh.client.Delete(ctx, &metricsDS); err != nil {
			return fmt.Errorf("failed to delete metrics exporter daemonset %s: %v", dsName, err)
		}
	}

	// Handle Secret deletion
	metricsSecret := v1.Secret{}
	secretName := types.NamespacedName{
		Namespace: nwConfig.Namespace,
		Name:      nwConfig.Name + "-" + metricsexporter.ExporterName,
	}
	if err := dcrh.client.Get(ctx, secretName, &metricsSecret); err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to get metrics exporter secret %s: %v", secretName, err)
		}
	} else {
		logger.Info("deleting metrics exporter secret", "secret", secretName)
		if err := dcrh.client.Delete(ctx, &metricsSecret); err != nil {
			return fmt.Errorf("failed to delete metrics exporter secret %s: %v", secretName, err)
		}
	}

	return nil
}

func (dcrh *networkConfigReconcilerHelper) finalizeCNIPlugins(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig) error {
	logger := log.FromContext(ctx)

	cniPluginsDS := appsv1.DaemonSet{}
	dsName := types.NamespacedName{
		Namespace: nwConfig.Namespace,
		Name:      fmt.Sprintf("%s-%s", nwConfig.Name, secondarynetwork.CNIPluginsName),
	}

	if err := dcrh.client.Get(ctx, dsName, &cniPluginsDS); err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to get CNI plugins daemonset %s: %v", dsName, err)
		}
	} else {
		logger.Info("deleting CNI plugins daemonset", "daemonset", dsName)
		if err := dcrh.client.Delete(ctx, &cniPluginsDS); err != nil {
			return fmt.Errorf("failed to delete CNI plugins daemonset %s: %v", dsName, err)
		}
	}

	return nil
}

func (dcrh *networkConfigReconcilerHelper) finalizeUpgradeWorkers(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig, nodes *v1.NodeList) error {
	logger := log.FromContext(ctx)
	label := dcrh.workerMgr.GetWorkReadyLabel(types.NamespacedName{Namespace: nwConfig.Namespace, Name: nwConfig.Name})
	for _, node := range nodes.Items {
		// Check if work pod exists and delete it
		podName := utils.GetUpgradeWorkerPodName(nwConfig, node.Name)
		workPod := &v1.Pod{}
		if err := dcrh.client.Get(ctx, types.NamespacedName{
			Namespace: nwConfig.Namespace,
			Name:      podName,
		}, workPod); err == nil {
			// Check if this is a work pod
			// for the undo pod, we won't delete it here
			// let it load the amdgpu back
			if workPod.Labels != nil && workPod.Labels[utils.WorkerActionLabelKey] == utils.WorkAction {
				logger.Info("Deleting work pod", "pod", podName, "node", node.Name)
				if err := dcrh.client.Delete(ctx, workPod); err != nil && !k8serrors.IsNotFound(err) {
					logger.Error(err, fmt.Sprintf("Failed to delete work pod %s on node %s", podName, node.Name))
					return err
				}
			}
		} else if !k8serrors.IsNotFound(err) {
			logger.Error(err, fmt.Sprintf("Failed to get pod %s on node %s", podName, node.Name))
			return err
		}

		if retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			nodeObj := &v1.Node{}
			if err := dcrh.client.Get(ctx, client.ObjectKey{Name: node.Name}, nodeObj); err != nil {
				return err
			}
			nodeObjCopy := nodeObj.DeepCopy()
			delete(nodeObj.Labels, label)
			return dcrh.client.Patch(ctx, nodeObj, client.MergeFrom(nodeObjCopy))
		}); retryErr != nil {
			logger.Error(retryErr, fmt.Sprintf("Node: %v. Failed to remove work ready label from node %s", node.Name, node.Name))
			return retryErr
		}
	}
	return nil
}

func (dcrh *networkConfigReconcilerHelper) finalizeDevicePlugin(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig) error {
	logger := log.FromContext(ctx)

	devPl := appsv1.DaemonSet{}
	dsName := types.NamespacedName{
		Namespace: nwConfig.Namespace,
		Name:      fmt.Sprintf("%s-%s", nwConfig.Name, deviceplugin.DevicePluginName),
	}

	if err := dcrh.client.Get(ctx, dsName, &devPl); err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to get device-plugin daemonset %s: %v", dsName, err)
		}
	} else {
		logger.Info("deleting device-plugin daemonset", "daemonset", dsName)
		if err := dcrh.client.Delete(ctx, &devPl); err != nil {
			return fmt.Errorf("failed to delete device-plugin daemonset %s: %v", dsName, err)
		}
	}

	return nil
}

func (dcrh *networkConfigReconcilerHelper) finalizeNodeLabeller(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig) error {
	logger := log.FromContext(ctx)

	nlDS := appsv1.DaemonSet{}
	dsName := types.NamespacedName{
		Namespace: nwConfig.Namespace,
		Name:      fmt.Sprintf("%s-%s", nwConfig.Name, nodelabeller.NodeLabellerName),
	}

	if err := dcrh.client.Get(ctx, dsName, &nlDS); err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to get node-labeller daemonset %s: %v", dsName, err)
		}
	} else {
		logger.Info("deleting node-labeller daemonset", "daemonset", dsName)
		if err := dcrh.client.Delete(ctx, &nlDS); err != nil {
			return fmt.Errorf("failed to delete node-labeller daemonset %s: %v", dsName, err)
		}
	}

	return nil
}

/*---- TO be Enabled later
func (dcrh *networkConfigReconcilerHelper) finalizeTestRunner(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig, nodes *v1.NodeList) error {
	logger := log.FromContext(ctx)

	trDS := appsv1.DaemonSet{}
	dsName := types.NamespacedName{
		Namespace: nwConfig.Namespace,
		Name:      nwConfig.Name + "-" + testrunner.TestRunnerName,
	}

	if err := dcrh.client.Get(ctx, dsName, &trDS); err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to get test runner daemonset %s: %v", dsName, err)
		}
	} else {
		logger.Info("deleting test runner daemonset", "daemonset", dsName)
		if err := dcrh.client.Delete(ctx, &trDS); err != nil {
			return fmt.Errorf("failed to delete test runner daemonset %s: %v", dsName, err)
		}
	}

	// clean up test running node label in case test runner gets disabled during test run
	for _, node := range nodes.Items {
		// add retry logic here
		// in case Node resource is being updated by multiple clients concurrently
		if retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			updated := false
			nodeObj := &v1.Node{}
			if err := dcrh.client.Get(ctx, client.ObjectKey{Name: node.Name}, nodeObj); err != nil {
				return err
			}
			nodeObjCopy := nodeObj.DeepCopy()

			for k := range nodeObjCopy.Labels {
				if strings.HasPrefix(k, testRunnerNodeLabelPrefix) {
					delete(nodeObj.Labels, k)
					updated = true
				}
			}

			// use PATCH instead of UPDATE
			// to minimize the resource usage, compared to update the whole Node resource
			if updated {
				logger.Info(fmt.Sprintf("removing test runner labels in %v", nodeObj.Name))
				return dcrh.client.Patch(ctx, nodeObj, client.MergeFrom(nodeObjCopy))
			}

			return nil
		}); retryErr != nil {
			logger.Error(retryErr, fmt.Sprintf("failed to remove test runner labels from node %+v", node.Name))
		}
	}

	return nil
}

---*/

func (dcrh *networkConfigReconcilerHelper) finalizeConfigManager(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig) error {
	logger := log.FromContext(ctx)

	trDS := appsv1.DaemonSet{}
	dsName := types.NamespacedName{
		Namespace: nwConfig.Namespace,
		Name:      nwConfig.Name + "-" + configmanager.ConfigManagerName,
	}

	if err := dcrh.client.Get(ctx, dsName, &trDS); err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to get config manager daemonset %s: %v", dsName, err)
		}
	} else {
		logger.Info("deleting config manager daemonset", "daemonset", dsName)
		if err := dcrh.client.Delete(ctx, &trDS); err != nil {
			return fmt.Errorf("failed to delete config manager daemonset %s: %v", dsName, err)
		}
	}

	return nil
}

func (dcrh *networkConfigReconcilerHelper) finalizeNetworkConfig(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig, nodes *v1.NodeList) error {

	// finalize config manager before metrics exporter
	if err := dcrh.finalizeConfigManager(ctx, nwConfig); err != nil {
		return err
	}
	/*---- To be enabled later
		// finalize test runner before metrics exporter
		if err := dcrh.finalizeTestRunner(ctx, nwConfig, nodes); err != nil {
			return err
		}
	----*/

	logger := log.FromContext(ctx)

	// finalize metrics exporter and metrics service
	// this should be removed firstly
	// because the exporter is using processes that could occupy the network driver GSM
	if err := dcrh.finalizeMetricsExporter(ctx, nwConfig); err != nil {
		return err
	}

	// finalize device plugin
	if err := dcrh.finalizeDevicePlugin(ctx, nwConfig); err != nil {
		return err
	}

	// finalize node labeller
	if err := dcrh.finalizeNodeLabeller(ctx, nwConfig); err != nil {
		return err
	}

	// finalize secondary network plugins
	if err := dcrh.finalizeCNIPlugins(ctx, nwConfig); err != nil {
		return err
	}

	// finalize existing workers created for driver upgrade and related node labels
	// in case the NetworkConfig is deleted during driver upgrade
	if err := dcrh.finalizeUpgradeWorkers(ctx, nwConfig, nodes); err != nil {
		return err
	}

	// finalize KMM CR of managing out-of-tree kernel module
	mod := kmmv1beta1.Module{}
	namespacedName := types.NamespacedName{
		Namespace: nwConfig.Namespace,
		Name:      nwConfig.Name,
	}
	if err := dcrh.client.Get(ctx, namespacedName, &mod); err != nil {
		if k8serrors.IsNotFound(err) {
			// if KMM module CR is not found
			if nwConfig.Spec.Driver.Enable != nil && *nwConfig.Spec.Driver.Enable {
				logger.Info("module already deleted, removing finalizer", "module", namespacedName)
			} else {
				// driver disabled mode won't have KMM CR created
				// but it still requries the removal of node labels
				if err := dcrh.updateNodeLabels(ctx, nwConfig, nodes, true); err != nil {
					logger.Error(err, "failed to update node labels")
				}
			}
			nwConfigCopy := nwConfig.DeepCopy()
			controllerutil.RemoveFinalizer(nwConfig, networkConfigFinalizer)
			return dcrh.client.Patch(ctx, nwConfig, client.MergeFrom(nwConfigCopy))
		}
		// other types of error occurred
		return fmt.Errorf("failed to get the requested Module %s: %v", namespacedName, err)
	}

	// if KMM module CR is found
	logger.Info("deleting KMM Module", "module", namespacedName)
	if err := dcrh.client.Delete(ctx, &mod); err != nil {
		return fmt.Errorf("failed to delete the requested Module: %s: %v", namespacedName, err)
	}
	if err := dcrh.updateNodeLabels(ctx, nwConfig, nodes, true); err != nil {
		logger.Error(err, "failed to update node labels")
	}

	// Update nodeAssignments after NetworkConfig status update
	dcrh.updateNodeAssignments(namespacedName.String(), nodes, true)

	return nil
}

func (dcrh *networkConfigReconcilerHelper) handleBuildConfigMap(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig, nodes *v1.NodeList) error {
	logger := log.FromContext(ctx)
	if nwConfig.Spec.Driver.Enable == nil || !*nwConfig.Spec.Driver.Enable {
		logger.Info("skip handling build config map as KMM driver mode is disabled")
		return nil
	}
	if nodes == nil || len(nodes.Items) == 0 {
		return fmt.Errorf("no nodes found for the label selector %s", kmmmodule.MapToLabelSelector(nwConfig.Spec.Selector))
	}

	savedCMName := map[string]bool{}
	buildOK := true
	for _, node := range nodes.Items {
		osName, err := kmmmodule.GetOSName(node, nwConfig)
		if err != nil {
			return fmt.Errorf("invalid node %s, err: %v", node.Name, err)
		}
		cmName := kmmmodule.GetCMName(osName, nwConfig)
		if savedCMName[cmName] {
			// already saved a docker file for the OS-Version combo
			continue
		}

		buildDockerfileCM := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: nwConfig.Namespace,
				Name:      cmName,
			},
		}

		opRes, err := controllerutil.CreateOrPatch(ctx, dcrh.client, buildDockerfileCM, func() error {
			return dcrh.kmmHandler.SetBuildConfigMapAsDesired(buildDockerfileCM, nwConfig)
		})

		if err == nil {
			logger.Info("Reconciled KMM build dockerfile ConfigMap", "name", buildDockerfileCM.Name, "result", opRes)
		} else {
			buildOK = false
			logger.Error(err, "error reconciling KMM build dockerfile ConfigMap", "name", buildDockerfileCM.Name, "result", opRes)
		}

		savedCMName[cmName] = true
	}

	if !buildOK {
		return errors.New("error reconciling KMM build dockerfile ConfigMap")
	}
	return nil
}

func (dcrh *networkConfigReconcilerHelper) handleKMMModule(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig, nodes *v1.NodeList) error {
	// the newly created KMM Module will always has the same namespace and name as its parent NetworkConfig
	kmmMod := &kmmv1beta1.Module{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: nwConfig.Namespace,
			Name:      nwConfig.Name,
		},
	}
	logger := log.FromContext(ctx)

	if nwConfig.Spec.Driver.Enable != nil && *nwConfig.Spec.Driver.Enable {
		opRes, err := controllerutil.CreateOrPatch(ctx, dcrh.client, kmmMod, func() error {
			return dcrh.kmmHandler.SetKMMModuleAsDesired(ctx, kmmMod, nwConfig, nodes)
		})

		if err == nil {
			logger.Info("Reconciled KMM Module", "name", kmmMod.Name, "result", opRes)
		}
		return err
	}
	logger.Info("skip handling KMM module as KMM driver mode is disabled")
	// if driver mode switched from enable to disable
	// we won't delete the existing KMM module

	return nil
}

func (dcrh *networkConfigReconcilerHelper) handleDevicePlugin(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig) error {
	logger := log.FromContext(ctx)
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: nwConfig.Namespace,
			Name:      fmt.Sprintf("%s-%s", nwConfig.Name, deviceplugin.DevicePluginName)},
	}

	dpOut := dpinternal.GenerateCommonDevicePluginSpec(nwConfig)
	opRes, err := controllerutil.CreateOrPatch(ctx, dcrh.client, ds, func() error {
		scheme, dcrhErr := dcrh.devicepluginHandler.SetDevicePluginAsDesired(ds, dpOut)
		if dcrhErr != nil {
			return dcrhErr
		}
		// Probably can switch to storing "scheme" in NetworkConfigReconciler struct
		return controllerutil.SetControllerReference(nwConfig, ds, scheme)
	})
	if err != nil {
		return err
	}
	logger.Info("Reconciled device-plugin", "namespace", ds.Namespace, "name", ds.Name, "result", opRes)

	return nil
}

func (dcrh *networkConfigReconcilerHelper) handleKMMVersionLabel(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig, nodes *v1.NodeList) error {
	// label corresponding node with given kmod version
	// so that KMM could manage the upgrade by watching the node's version label change
	if nwConfig.Spec.Driver.Enable != nil && *nwConfig.Spec.Driver.Enable {
		err := dcrh.kmmHandler.SetNodeVersionLabelAsDesired(ctx, nwConfig, nodes)
		if err != nil {
			return fmt.Errorf("failed to update node version label for NetworkConfig %s/%s: %v", nwConfig.Namespace, nwConfig.Name, err)
		}
	}
	return nil
}

func (dcrh *networkConfigReconcilerHelper) handleNodeLabeller(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig, nodes *v1.NodeList, isOpenShift bool) error {
	logger := log.FromContext(ctx)

	nlFullName := fmt.Sprintf("%s-%s", nwConfig.Name, nlinternal.NodeLabellerNameSuffix)
	if nwConfig.Spec.DevicePlugin.EnableNodeLabeller == nil || !*nwConfig.Spec.DevicePlugin.EnableNodeLabeller {
		if err := dcrh.finalizeNodeLabeller(ctx, nwConfig); err != nil {
			return err
		}

		// clean up node labeller's label when node labeller is disabled
		// if no label need to be removed, updateNodeLabels won't send request
		if err := dcrh.updateNodeLabels(ctx, nwConfig, nodes, false); err != nil {
			logger.Error(err, "failed to remove node labeller's labels when node labeller is disabled")
		}
		logger.Info("skip handling node labeller as it is disbaled", "namespace", nwConfig.Namespace, "name", nlFullName)
		return nil
	}

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: nwConfig.Namespace, Name: nlFullName},
	}
	nlOut := nlinternal.GenerateCommonNodeLabellerSpec(nwConfig, isOpenShift)
	opRes, err := controllerutil.CreateOrPatch(ctx, dcrh.client, ds, func() error {
		scheme, dcrhErr := dcrh.nlHandler.SetNodeLabellerAsDesired(ds, nlOut)
		if dcrhErr != nil {
			return dcrhErr
		}
		// Probably can switch to storing "scheme" in NetworkConfigReconciler struct
		return controllerutil.SetControllerReference(nwConfig, ds, scheme)
	})

	if err != nil {
		return err
	}

	logger.Info("Reconciled node labeller", "namespace", ds.Namespace, "name", ds.Name, "result", opRes)

	// todo: temp. cleanup labels set by node-labeller
	// not required once label cleanup is added in node-labeller
	nodeLabels := func() string {
		// nodes without network, kmm, dev-plugin //GSM
		sel := []string{
			"! " + utils.NodeFeatureLabelAmdNic,
			"! " + utils.NodeFeatureLabelAmdVNic,
			"! " + labels.GetKernelModuleReadyNodeLabel(nwConfig.Namespace, nwConfig.Name),
			"! " + labels.GetDevicePluginNodeLabel(nwConfig.Namespace, nwConfig.Name),
		}

		for k, v := range nwConfig.Spec.Selector {
			if k == utils.NodeFeatureLabelAmdNic ||
				k == utils.NodeFeatureLabelAmdVNic { // skip
				continue
			}
			sel = append(sel, fmt.Sprintf("%s=%s", k, v))
		}
		return strings.Join(sel, ",")
	}()

	its, err := kmmmodule.GetK8SNodes(nodeLabels)
	if err != nil {
		logger.Info("failed to get node list ", err)
		return nil
	}
	logger.Info(fmt.Sprintf("select (%v) found %v nodes", nodeLabels, len(its.Items)))

	if err := dcrh.updateNodeLabels(ctx, nwConfig, its, false); err != nil {
		logger.Error(err, "failed to update node labels")
	}
	return nil
}

func (dcrh *networkConfigReconcilerHelper) handleModuleUpgrade(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig, nodes *v1.NodeList, delete bool) (ctrl.Result, error) {
	if delete {
		return dcrh.upgradeMgrHandler.HandleDelete(ctx, nwConfig, nodes)
	}
	return dcrh.upgradeMgrHandler.HandleUpgrade(ctx, nwConfig, nodes)
}

func (dcrh *networkConfigReconcilerHelper) handleMetricsExporter(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig) error {
	logger := log.FromContext(ctx)
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: nwConfig.Namespace, Name: nwConfig.Name + "-" + metricsexporter.ExporterName},
	}

	// delete if disabled
	if nwConfig.Spec.MetricsExporter.Enable == nil || !*nwConfig.Spec.MetricsExporter.Enable {
		return dcrh.finalizeMetricsExporter(ctx, nwConfig)
	}

	mxOut := expinternal.GenerateCommonExporterSpec(nwConfig)
	if nwConfig.Spec.MetricsExporter.RbacConfig.StaticAuthorization != nil && nwConfig.Spec.MetricsExporter.RbacConfig.StaticAuthorization.Enable {
		secret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: nwConfig.Namespace,
				Name:      mxOut.RbacConfig.StaticAuthorization.SecretName,
			},
		}
		opRes, err := controllerutil.CreateOrPatch(ctx, dcrh.client, secret, func() error {
			scheme, dcrhErr := dcrh.metricsHandler.SetStaticAuthSecretAsDesired(secret, mxOut)
			if dcrhErr != nil {
				return dcrhErr
			}
			// Probably can switch to storing "scheme" in NetworkConfigReconciler struct
			return controllerutil.SetControllerReference(nwConfig, secret, scheme)
		})
		if err != nil {
			return err
		}
		logger.Info("Reconciled static auth secret", "namespace", secret.Namespace, "name", secret.Name, "result", opRes)
	}

	opRes, err := controllerutil.CreateOrPatch(ctx, dcrh.client, ds, func() error {
		scheme, dcrhErr := dcrh.metricsHandler.SetMetricsExporterAsDesired(ds, mxOut)
		if dcrhErr != nil {
			return dcrhErr
		}
		// Probably can switch to storing "scheme" in NetworkConfigReconciler struct
		return controllerutil.SetControllerReference(nwConfig, ds, scheme)
	})
	if err != nil {
		return err
	}
	logger.Info("Reconciled metrics exporter", "namespace", ds.Namespace, "name", ds.Name, "result", opRes)

	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: nwConfig.Namespace, Name: nwConfig.Name + "-" + metricsexporter.ExporterName},
	}
	opRes, err = controllerutil.CreateOrPatch(ctx, dcrh.client, svc, func() error {
		scheme, dcrhErr := dcrh.metricsHandler.SetMetricsServiceAsDesired(svc, mxOut)
		if dcrhErr != nil {
			return dcrhErr
		}
		// Probably can switch to storing "scheme" in NetworkConfigReconciler struct
		return controllerutil.SetControllerReference(nwConfig, svc, scheme)
	})

	if err != nil {
		return err
	}
	logger.Info("Reconciled metrics service", "namespace", svc.Namespace, "name", svc.Name, "result", opRes)

	if utils.IsPrometheusServiceMonitorEnable(nwConfig) {
		// Create or update the ServiceMonitor resource
		sm := &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: nwConfig.Namespace,
				Name:      nwConfig.Name + "-" + metricsexporter.ExporterName,
			},
		}

		opRes, err = controllerutil.CreateOrPatch(ctx, dcrh.client, sm, func() error {
			scheme, dcrhErr := dcrh.metricsHandler.SetServiceMonitorAsDesired(sm, mxOut)
			if dcrhErr != nil {
				return dcrhErr
			}
			// Probably can switch to storing "scheme" in NetworkConfigReconciler struct
			return controllerutil.SetControllerReference(nwConfig, sm, scheme)
		})

		if err != nil {
			return err
		}
		logger.Info("Reconciled ServiceMonitor", "namespace", sm.Namespace, "name", sm.Name, "result", opRes)
	} else {
		// Delete any existing ServiceMonitor if the feature is disabled
		sm := &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: nwConfig.Namespace,
				Name:      nwConfig.Name + "-" + metricsexporter.ExporterName,
			},
		}

		err = dcrh.client.Get(ctx, client.ObjectKeyFromObject(sm), sm)
		if err == nil {
			// ServiceMonitor exists but the feature is disabled, delete it
			logger.Info("ServiceMonitor feature is disabled, removing existing ServiceMonitor",
				"namespace", sm.Namespace, "name", sm.Name)
			if err := dcrh.client.Delete(ctx, sm); err != nil && !k8serrors.IsNotFound(err) {
				return fmt.Errorf("failed to delete ServiceMonitor: %v", err)
			}
		} else if !k8serrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			// Some other error occurred
			return fmt.Errorf("failed to get ServiceMonitor: %v", err)
		}
		// If error is IsNotFound or NoMatch (CRD not available), then there's nothing to delete
	}

	return nil
}

func (dcrh *networkConfigReconcilerHelper) handleSecondaryNetwork(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig) error {
	logger := log.FromContext(ctx)

	if nwConfig.Spec.SecondaryNetwork.CniPlugins != nil {
		ds := &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{Namespace: nwConfig.Namespace, Name: nwConfig.Name + "-" + secondarynetwork.CNIPluginsName},
		}

		// delete if disabled
		if nwConfig.Spec.SecondaryNetwork.CniPlugins.Enable == nil || !*nwConfig.Spec.SecondaryNetwork.CniPlugins.Enable {
			return dcrh.finalizeCNIPlugins(ctx, nwConfig)
		}

		opRes, err := controllerutil.CreateOrPatch(ctx, dcrh.client, ds, func() error {
			scheme, dcrhErr := dcrh.secondaryNetworkHandler.SetCNIPluginsAsDesired(nwConfig.Name, ds, nwConfig.Spec.SecondaryNetwork.CniPlugins, nwConfig.Spec.Selector)
			if dcrhErr != nil {
				return dcrhErr
			}
			return controllerutil.SetControllerReference(nwConfig, ds, scheme)
		})
		if err != nil {
			return err
		}
		logger.Info("Reconciled CNI plugins", "namespace", ds.Namespace, "name", ds.Name, "result", opRes)
	}

	return nil
}

/*
---- To be enabled later

	func (dcrh *networkConfigReconcilerHelper) handleTestRunner(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig, nodes *v1.NodeList) error {
		logger := log.FromContext(ctx)
		ds := &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{Namespace: nwConfig.Namespace, Name: nwConfig.Name + "-" + testrunner.TestRunnerName},
		}

		// delete if disabled
		// if metrics exporter is disabled, disable the test runner as well
		// because the test runner's auto unhealthy Network watch functionality is depending on metrics exporter
		if (nwConfig.Spec.TestRunner.Enable == nil || !*nwConfig.Spec.TestRunner.Enable) ||
			(nwConfig.Spec.MetricsExporter.Enable == nil || !*nwConfig.Spec.MetricsExporter.Enable) {
			return dcrh.finalizeTestRunner(ctx, nwConfig, nodes)
		}

		opRes, err := controllerutil.CreateOrPatch(ctx, dcrh.client, ds, func() error {
			return dcrh.testrunnerHandler.SetTestRunnerAsDesired(ds, nwConfig)
		})
		if err != nil {
			return err
		}
		logger.Info("Reconciled test runner", "namespace", ds.Namespace, "name", ds.Name, "result", opRes)

		return nil
	}

---
*/
func (dcrh *networkConfigReconcilerHelper) handleConfigManager(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig) error {
	logger := log.FromContext(ctx)
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: nwConfig.Namespace, Name: nwConfig.Name + "-" + configmanager.ConfigManagerName},
	}

	// delete if disabled
	if nwConfig.Spec.ConfigManager.Enable == nil || !*nwConfig.Spec.ConfigManager.Enable {
		return dcrh.finalizeConfigManager(ctx, nwConfig)
	}

	cmOut := cminternal.GenerateCommonConfigManagerSpec(nwConfig)
	opRes, err := controllerutil.CreateOrPatch(ctx, dcrh.client, ds, func() error {
		scheme, dcrhErr := dcrh.configmanagerHandler.SetConfigManagerAsDesired(ds, cmOut)
		if dcrhErr != nil {
			return dcrhErr
		}
		// Probably can switch to storing "scheme" in NetworkConfigReconciler struct
		return controllerutil.SetControllerReference(nwConfig, ds, scheme)
	})
	if err != nil {
		return err
	}
	logger.Info("Reconciled config manager", "namespace", ds.Namespace, "name", ds.Name, "result", opRes)

	return nil
}

func (dcrh *networkConfigReconcilerHelper) updateNodeLabels(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig, nodes *v1.NodeList, isFinalizer bool) error {
	logger := log.FromContext(ctx)
	labelKey, _ := kmmmodule.GetVersionLabelKV(nwConfig)

	for _, node := range nodes.Items {
		// add retry logic here
		// in case Node resource is being updated by multiple clients concurrently
		if retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			updated := false
			nodeObj := &v1.Node{}
			if err := dcrh.client.Get(ctx, client.ObjectKey{Name: node.Name}, nodeObj); err != nil {
				return err
			}
			nodeObjCopy := nodeObj.DeepCopy()

			if isFinalizer {
				if _, ok := nodeObj.Labels[labelKey]; ok {
					delete(nodeObj.Labels, labelKey)
					updated = true
				}
			}

			for k := range nodeObjCopy.Labels {
				if strings.HasPrefix(k, "beta.amd.com") ||
					strings.HasPrefix(k, "amd.com") {
					delete(nodeObj.Labels, k)
					updated = true
				}
			}

			// use PATCH instead of UPDATE
			// to minimize the resource usage, compared to update the whole Node resource
			if updated {
				logger.Info(fmt.Sprintf("updating node-labeller labels in %v", nodeObj.Name))
				return dcrh.client.Patch(ctx, nodeObj, client.MergeFrom(nodeObjCopy))
			}

			return nil
		}); retryErr != nil {
			logger.Error(retryErr, fmt.Sprintf("failed to remove labels from node %+v", node.Name))
		}
	}
	return nil
}

func (dcrh *networkConfigReconcilerHelper) validateNodeAssignments(namespacedName string, nodes *v1.NodeList) error {
	var err error

	for _, node := range nodes.Items {
		val, ok := dcrh.nodeAssignments[node.Name]
		if ok && val != namespacedName {
			err = fmt.Errorf("node %s already assigned to NetworkConfig %s, cannot re-assign to %s", node.Name, val, namespacedName)
			break
		}
	}

	return err
}

func (dcrh *networkConfigReconcilerHelper) buildNodeAssignments(networkConfigList *amdv1alpha1.NetworkConfigList) error {
	if networkConfigList == nil {
		return nil
	}

	isReady := func(nwConfig *amdv1alpha1.NetworkConfig) bool {
		ready := dcrh.conditionUpdater.GetReadyCondition(nwConfig)
		if ready == nil {
			return false
		}
		return ready.Status == metav1.ConditionTrue
	}

	for _, nwConfig := range networkConfigList.Items {
		if isReady(&nwConfig) {
			namespacedName := types.NamespacedName{
				Namespace: nwConfig.Namespace,
				Name:      nwConfig.Name,
			}

			nodeItems := []v1.Node{}
			for node := range nwConfig.Status.NodeModuleStatus {
				nodeItems = append(nodeItems, v1.Node{ObjectMeta: metav1.ObjectMeta{Name: node}})
			}
			err := dcrh.validateNodeAssignments(namespacedName.String(), &v1.NodeList{Items: nodeItems})
			if err != nil {
				return err
			}
			dcrh.updateNodeAssignments(namespacedName.String(), &v1.NodeList{Items: nodeItems}, false)
		}
	}

	return nil
}

func (dcrh *networkConfigReconcilerHelper) updateNodeAssignments(namespacedName string, nodes *v1.NodeList, isFinalizer bool) {
	if isFinalizer {
		if nodes != nil {
			for _, node := range nodes.Items {
				delete(dcrh.nodeAssignments, node.Name)
			}
		} else {
			for k, v := range dcrh.nodeAssignments {
				if v == namespacedName {
					delete(dcrh.nodeAssignments, k)
				}
			}
		}
		return
	}

	for _, node := range nodes.Items {
		dcrh.nodeAssignments[node.Name] = namespacedName
	}
}

func (dcrh *networkConfigReconcilerHelper) setCondition(ctx context.Context, condition string, nwConfig *amdv1alpha1.NetworkConfig, status metav1.ConditionStatus, reason string, message string) error {
	switch condition {
	case conditions.ConditionTypeReady:
		dcrh.conditionUpdater.SetReadyCondition(nwConfig, status, reason, message)
		return dcrh.updateNetworkConfigStatus(ctx, nwConfig)
	case conditions.ConditionTypeError:
		dcrh.conditionUpdater.SetErrorCondition(nwConfig, status, reason, message)
		return dcrh.updateNetworkConfigStatus(ctx, nwConfig)
	}
	return fmt.Errorf("Condition %s not supported", condition)
}

func (dcrh *networkConfigReconcilerHelper) deleteCondition(ctx context.Context, condition string, nwConfig *amdv1alpha1.NetworkConfig) error {
	switch condition {
	case conditions.ConditionTypeReady:
		dcrh.conditionUpdater.DeleteReadyCondition(nwConfig)
		return dcrh.updateNetworkConfigStatus(ctx, nwConfig)
	case conditions.ConditionTypeError:
		dcrh.conditionUpdater.DeleteErrorCondition(nwConfig)
		return dcrh.updateNetworkConfigStatus(ctx, nwConfig)
	}
	return fmt.Errorf("Condition %s not supported", condition)
}

func (dcrh *networkConfigReconcilerHelper) validateNetworkConfig(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig) []string {
	// Validate only if the spec has changed since the last successful validation
	if nwConfig.Generation != nwConfig.Status.ObservedGeneration {
		return dcrh.validator.ValidateNetworkConfigAll(ctx, dcrh.client, nwConfig)
	}
	return nil
}
