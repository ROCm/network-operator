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
	"fmt"

	"github.com/ROCm/common-infra-operator/pkg/metricsexporter"
	amdv1alpha1 "github.com/ROCm/network-operator/api/v1alpha1"
	mock_client "github.com/ROCm/network-operator/internal/client"
	"github.com/ROCm/network-operator/internal/kmmmodule"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	kmmv1beta1 "github.com/rh-ecosystem-edge/kernel-module-management/api/v1beta1"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	nwConfigName      = "nwConfigName"
	nwConfigNamespace = "nwConfigNamespace"
)

var (
	testNodeList = &v1.NodeList{
		Items: []v1.Node{
			{
				TypeMeta: metav1.TypeMeta{
					Kind: "Node",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "unit-test-node",
				},
				Spec: v1.NodeSpec{},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						Architecture:            "amd64",
						ContainerRuntimeVersion: "containerd://1.7.19",
						KernelVersion:           "6.8.0-40-generic",
						KubeProxyVersion:        "v1.30.3",
						KubeletVersion:          "v1.30.3",
						OperatingSystem:         "linux",
						OSImage:                 "Ubuntu 22.04.3 LTS",
					},
				},
			},
		},
	}
)

var _ = Describe("getLabelsPerModules", func() {
	var (
		kubeClient *mock_client.MockClient
		dcrh       networkConfigReconcilerHelperAPI
	)

	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		kubeClient = mock_client.NewMockClient(ctrl)
		dcrh = newNetworkConfigReconcilerHelper(kubeClient, nil, nil, nil, nil, nil, nil, nil, nil)
	})

	ctx := context.Background()
	nn := types.NamespacedName{
		Name:      nwConfigName,
		Namespace: nwConfigNamespace,
	}

	It("good flow", func() {
		expectedNwConfig := amdv1alpha1.NetworkConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nn.Name,
				Namespace: nn.Namespace,
			},
		}
		kubeClient.EXPECT().Get(ctx, nn, gomock.Any()).Do(
			func(_ interface{}, _ interface{}, nwConfig *amdv1alpha1.NetworkConfig, _ ...client.GetOption) {
				nwConfig.Name = nn.Name
				nwConfig.Namespace = nn.Namespace
			},
		)
		res, err := dcrh.getRequestedNetworkConfig(ctx, nn)
		Expect(err).ToNot(HaveOccurred())
		Expect(*res).To(Equal(expectedNwConfig))
	})

	It("error flow", func() {
		kubeClient.EXPECT().Get(ctx, nn, gomock.Any()).Return(fmt.Errorf("some error"))

		res, err := dcrh.getRequestedNetworkConfig(ctx, nn)
		Expect(err).To(HaveOccurred())
		Expect(res).To(BeNil())
	})
})

var _ = Describe("setFinalizer", func() {
	var (
		kubeClient *mock_client.MockClient
		dcrh       networkConfigReconcilerHelperAPI
	)

	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		kubeClient = mock_client.NewMockClient(ctrl)
		dcrh = newNetworkConfigReconcilerHelper(kubeClient, nil, nil, nil, nil, nil, nil, nil, nil)
	})

	ctx := context.Background()

	It("good flow", func() {
		nwConfig := &amdv1alpha1.NetworkConfig{}

		kubeClient.EXPECT().Patch(ctx, gomock.Any(), gomock.Any()).Return(nil)

		err := dcrh.setFinalizer(ctx, nwConfig)
		Expect(err).ToNot(HaveOccurred())

		err = dcrh.setFinalizer(ctx, nwConfig)
		Expect(err).ToNot(HaveOccurred())
	})

	It("error flow", func() {
		nwConfig := &amdv1alpha1.NetworkConfig{}

		kubeClient.EXPECT().Patch(ctx, gomock.Any(), gomock.Any()).Return(fmt.Errorf("some error"))

		err := dcrh.setFinalizer(ctx, nwConfig)
		Expect(err).To(HaveOccurred())
	})
})

var _ = PDescribe("finalizeNetworkConfig", func() {
	var (
		kubeClient *mock_client.MockClient
		dcrh       networkConfigReconcilerHelperAPI
	)

	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		kubeClient = mock_client.NewMockClient(ctrl)
		dcrh = newNetworkConfigReconcilerHelper(kubeClient, nil, nil, nil, nil, nil, nil, nil, nil)
	})

	ctx := context.Background()
	driverEnable := true
	nwConfig := &amdv1alpha1.NetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nwConfigName,
			Namespace: nwConfigNamespace,
		},
		Spec: amdv1alpha1.NetworkConfigSpec{
			Driver: amdv1alpha1.DriverSpec{
				Enable: &driverEnable,
			},
		},
	}

	nodeLabellerNN := types.NamespacedName{
		Name:      nwConfigName + "-node-labeller",
		Namespace: nwConfigNamespace,
	}

	devPluginNN := types.NamespacedName{
		Name:      nwConfigName + "-device-plugin",
		Namespace: nwConfigNamespace,
	}

	metricsNN := types.NamespacedName{
		Name:      nwConfigName + "-" + metricsexporter.ExporterName,
		Namespace: nwConfigNamespace,
	}

	nn := types.NamespacedName{
		Name:      nwConfigName,
		Namespace: nwConfigNamespace,
	}

	testNodeNN := types.NamespacedName{
		Name: "unit-test-node",
	}

	It("failed to get NodeLabeller daemonset", func() {
		statusErr := &k8serrors.StatusError{
			ErrStatus: metav1.Status{
				Reason: metav1.StatusReasonNotFound,
			},
		}

		kubeClient.EXPECT().Get(ctx, devPluginNN, gomock.Any()).Return(statusErr).Times(1)
		kubeClient.EXPECT().Get(ctx, testNodeNN, gomock.Any()).Return(nil).Times(1)
		kubeClient.EXPECT().Get(ctx, metricsNN, gomock.Any()).Return(statusErr).Times(4)
		kubeClient.EXPECT().Get(ctx, nodeLabellerNN, gomock.Any()).Return(fmt.Errorf("some error"))

		err := dcrh.finalizeNetworkConfig(ctx, nwConfig, testNodeList)
		Expect(err).To(HaveOccurred())
	})

	It("node metrics daemonset exists", func() {
		statusErr := &k8serrors.StatusError{
			ErrStatus: metav1.Status{
				Reason: metav1.StatusReasonNotFound,
			},
		}

		gomock.InOrder(
			kubeClient.EXPECT().Get(ctx, testNodeNN, gomock.Any()).Return(nil).Times(1),
			kubeClient.EXPECT().Get(ctx, metricsNN, gomock.Any()).Return(statusErr).Times(4),
			kubeClient.EXPECT().Get(ctx, devPluginNN, gomock.Any()).Return(statusErr).Times(1),
			kubeClient.EXPECT().Get(ctx, nodeLabellerNN, gomock.Any()).Return(k8serrors.NewNotFound(schema.GroupResource{}, "dsName")),
			kubeClient.EXPECT().Get(ctx, nn, gomock.Any()).Return(nil),
			kubeClient.EXPECT().Delete(ctx, gomock.Any()).Return(nil),
			kubeClient.EXPECT().Get(ctx, testNodeNN, gomock.Any()).Return(nil),
		)

		err := dcrh.finalizeNetworkConfig(ctx, nwConfig, testNodeList)
		Expect(err).To(BeNil())
	})

	It("failed to get KMM Module", func() {
		statusErr := &k8serrors.StatusError{
			ErrStatus: metav1.Status{
				Reason: metav1.StatusReasonNotFound,
			},
		}

		gomock.InOrder(
			kubeClient.EXPECT().Get(ctx, testNodeNN, gomock.Any()).Return(nil).Times(1),
			kubeClient.EXPECT().Get(ctx, metricsNN, gomock.Any()).Return(statusErr).Times(4),
			kubeClient.EXPECT().Get(ctx, devPluginNN, gomock.Any()).Return(statusErr).Times(1),
			kubeClient.EXPECT().Get(ctx, nodeLabellerNN, gomock.Any()).Return(k8serrors.NewNotFound(schema.GroupResource{}, "dsName")),
			kubeClient.EXPECT().Get(ctx, nn, gomock.Any()).Return(fmt.Errorf("some error")),
		)

		err := dcrh.finalizeNetworkConfig(ctx, nwConfig, testNodeList)
		Expect(err).To(HaveOccurred())
	})

	It("KMM module not found, removing finalizer", func() {
		statusErr := &k8serrors.StatusError{
			ErrStatus: metav1.Status{
				Reason: metav1.StatusReasonNotFound,
			},
		}

		expectedNwConfig := nwConfig.DeepCopy()
		expectedNwConfig.SetFinalizers([]string{})
		controllerutil.AddFinalizer(nwConfig, networkConfigFinalizer)

		gomock.InOrder(
			kubeClient.EXPECT().Get(ctx, testNodeNN, gomock.Any()).Return(nil).Times(1),
			kubeClient.EXPECT().Get(ctx, metricsNN, gomock.Any()).Return(statusErr).Times(4),
			kubeClient.EXPECT().Get(ctx, devPluginNN, gomock.Any()).Return(statusErr).Times(1),
			kubeClient.EXPECT().Get(ctx, nodeLabellerNN, gomock.Any()).Return(k8serrors.NewNotFound(schema.GroupResource{}, "dsName")),
			kubeClient.EXPECT().Get(ctx, nn, gomock.Any()).Return(k8serrors.NewNotFound(schema.GroupResource{}, "moduleName")),
			kubeClient.EXPECT().Patch(ctx, expectedNwConfig, gomock.Any()).Return(nil),
		)

		err := dcrh.finalizeNetworkConfig(ctx, nwConfig, testNodeList)
		Expect(err).ToNot(HaveOccurred())
	})

	It("KMM module found, deleting it", func() {
		statusErr := &k8serrors.StatusError{
			ErrStatus: metav1.Status{
				Reason: metav1.StatusReasonNotFound,
			},
		}

		mod := kmmv1beta1.Module{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nwConfigName,
				Namespace: nwConfigNamespace,
			},
		}

		expectedNwConfig := nwConfig.DeepCopy()
		expectedNwConfig.SetFinalizers([]string{})
		controllerutil.AddFinalizer(nwConfig, networkConfigFinalizer)

		gomock.InOrder(
			kubeClient.EXPECT().Get(ctx, testNodeNN, gomock.Any()).Return(nil).Times(1),
			kubeClient.EXPECT().Get(ctx, metricsNN, gomock.Any()).Return(statusErr).Times(4),
			kubeClient.EXPECT().Get(ctx, devPluginNN, gomock.Any()).Return(statusErr).Times(1),
			kubeClient.EXPECT().Get(ctx, nodeLabellerNN, gomock.Any()).Return(k8serrors.NewNotFound(schema.GroupResource{}, "dsName")),
			kubeClient.EXPECT().Get(ctx, nn, gomock.Any()).Do(
				func(_ interface{}, _ interface{}, mod *kmmv1beta1.Module, _ ...client.GetOption) {
					mod.Name = nn.Name
					mod.Namespace = nn.Namespace
				},
			),
			kubeClient.EXPECT().Delete(ctx, &mod).Return(nil),
			kubeClient.EXPECT().Get(ctx, testNodeNN, gomock.Any()).Return(nil),
		)

		err := dcrh.finalizeNetworkConfig(ctx, nwConfig, testNodeList)
		Expect(err).ToNot(HaveOccurred())
	})
})

var _ = PDescribe("handleKMMModule", func() {
	var (
		kubeClient *mock_client.MockClient
		kmmHelper  *kmmmodule.MockKMMModuleAPI
		dcrh       networkConfigReconcilerHelperAPI
	)

	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		kubeClient = mock_client.NewMockClient(ctrl)
		kmmHelper = kmmmodule.NewMockKMMModuleAPI(ctrl)
		dcrh = newNetworkConfigReconcilerHelper(kubeClient, kmmHelper, nil, nil, nil, nil, nil, nil, nil)
	})

	ctx := context.Background()
	driverEnable := true
	nwConfig := &amdv1alpha1.NetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nwConfigName,
			Namespace: nwConfigNamespace,
		},
		Spec: amdv1alpha1.NetworkConfigSpec{
			Driver: amdv1alpha1.DriverSpec{
				Enable: &driverEnable,
			},
		},
	}

	It("KMM Module does not exist", func() {
		newMod := &kmmv1beta1.Module{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: nwConfig.Namespace,
				Name:      nwConfig.Name,
			},
		}
		gomock.InOrder(
			kubeClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Return(k8serrors.NewNotFound(schema.GroupResource{}, "whatever")),
			kmmHelper.EXPECT().SetKMMModuleAsDesired(ctx, newMod, nwConfig, testNodeList).Return(nil),

			kubeClient.EXPECT().Create(ctx, gomock.Any()).Return(nil),
		)

		err := dcrh.handleKMMModule(ctx, nwConfig, testNodeList)
		Expect(err).ToNot(HaveOccurred())
	})

	It("KMM Module exists", func() {
		existingMod := &kmmv1beta1.Module{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: nwConfig.Namespace,
				Name:      nwConfig.Name,
			},
		}
		gomock.InOrder(
			kubeClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Do(
				func(_ interface{}, _ interface{}, mod *kmmv1beta1.Module, _ ...client.GetOption) {
					mod.Name = nwConfig.Name
					mod.Namespace = nwConfig.Namespace
				},
			),
			kmmHelper.EXPECT().SetKMMModuleAsDesired(ctx, existingMod, nwConfig, testNodeList).Return(nil),
		)

		err := dcrh.handleKMMModule(ctx, nwConfig, testNodeList)
		Expect(err).ToNot(HaveOccurred())
	})
})

var _ = Describe("handleBuildConfigMap", func() {
	var (
		kubeClient *mock_client.MockClient
		kmmHelper  *kmmmodule.MockKMMModuleAPI
		dcrh       networkConfigReconcilerHelperAPI
	)

	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		kubeClient = mock_client.NewMockClient(ctrl)
		kmmHelper = kmmmodule.NewMockKMMModuleAPI(ctrl)
		dcrh = newNetworkConfigReconcilerHelper(kubeClient, kmmHelper, nil, nil, nil, nil, nil, nil, nil)
	})

	ctx := context.Background()
	driverEnable := true
	nwConfig := &amdv1alpha1.NetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nwConfigName,
			Namespace: nwConfigNamespace,
		},
		Spec: amdv1alpha1.NetworkConfigSpec{
			Driver: amdv1alpha1.DriverSpec{
				Enable: &driverEnable,
			},
		},
	}

	It("BuildConfig does not exist", func() {
		newBuildCM := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: nwConfig.Namespace,
				Name:      kmmmodule.GetCMName("ubuntu-22.04", nwConfig),
			},
		}
		gomock.InOrder(
			kubeClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Return(k8serrors.NewNotFound(schema.GroupResource{}, "whatever")),
			kmmHelper.EXPECT().SetBuildConfigMapAsDesired(newBuildCM, nwConfig).Return(nil),
			kubeClient.EXPECT().Create(ctx, gomock.Any()).Return(nil),
		)

		err := dcrh.handleBuildConfigMap(ctx, nwConfig, testNodeList)
		Expect(err).ToNot(HaveOccurred())
	})

	It("BuildConfig exists", func() {
		existingBuildCM := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: nwConfig.Namespace,
				Name:      kmmmodule.GetCMName("ubuntu-22.04", nwConfig),
			},
		}
		gomock.InOrder(
			kubeClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Do(
				func(_ interface{}, _ interface{}, buildCM *v1.ConfigMap, _ ...client.GetOption) {
					buildCM.Name = kmmmodule.GetCMName("ubuntu-22.04", nwConfig)
					buildCM.Namespace = nwConfig.Namespace
				},
			),
			kmmHelper.EXPECT().SetBuildConfigMapAsDesired(existingBuildCM, nwConfig).Return(nil),
		)

		err := dcrh.handleBuildConfigMap(ctx, nwConfig, testNodeList)
		Expect(err).ToNot(HaveOccurred())
	})
})

/***
//TODO uncomment enable after PR#2 merge
var _ = Describe("handleNodeLabeller", func() {
	var (
		kubeClient         *mock_client.MockClient
		nodeLabellerHelper *nodelabeller.MockNodeLabeller
		dcrh               networkConfigReconcilerHelperAPI
	)

	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		kubeClient = mock_client.NewMockClient(ctrl)
		nodeLabellerHelper = nodelabeller.NewMockNodeLabeller(ctrl)
		dcrh = newNetworkConfigReconcilerHelper(kubeClient, nil, nodeLabellerHelper, nil, nil, nil, nil)
	})

	ctx := context.Background()
	enableNodeLabeller := true
	nwConfig := &amdv1alpha1.NetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nwConfigName,
			Namespace: nwConfigNamespace,
		},
		Spec: amdv1alpha1.NetworkConfigSpec{
			DevicePlugin: amdv1alpha1.DevicePluginSpec{
				EnableNodeLabeller: &enableNodeLabeller,
			},
		},
	}

	It("NodeLabeller DaemonSet does not exist", func() {
		scheme := runtime.NewScheme()
		utilruntime.Must(amdv1alpha1.AddToScheme(scheme))
		newDS := &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{Namespace: nwConfig.Namespace, Name: nwConfig.Name + "-node-labeller"},
		}
		nlOut := utils.GenerateCommonNodeLabellerSpec(nwConfig)


		gomock.InOrder(
			kubeClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Return(k8serrors.NewNotFound(schema.GroupResource{}, "whatever")),
			nodeLabellerHelper.EXPECT().SetNodeLabellerAsDesired(newDS, nlOut).Return(scheme, nil),
			kubeClient.EXPECT().Create(ctx, gomock.Any()).Return(nil),
		)

		err := dcrh.handleNodeLabeller(ctx, nwConfig, testNodeList)
		Expect(err).ToNot(HaveOccurred())
	})

	It("NodeLabeller DaemonSet exists", func() {
		scheme := runtime.NewScheme()
		utilruntime.Must(amdv1alpha1.AddToScheme(scheme))
		existingDS := &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{Namespace: nwConfig.Namespace, Name: nwConfig.Name + "-node-labeller"},
		}
		nlOut := utils.GenerateCommonNodeLabellerSpec(nwConfig)


		gomock.InOrder(
			kubeClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Do(
				func(_ interface{}, _ interface{}, ds *appsv1.DaemonSet, _ ...client.GetOption) {
					ds.Name = nwConfig.Name + "-node-labeller"
					ds.Namespace = nwConfig.Namespace
				},
			),
			nodeLabellerHelper.EXPECT().SetNodeLabellerAsDesired(existingDS, nlOut).Return(scheme, nil),
		)

		kubeClient.EXPECT().Patch(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		err := dcrh.handleNodeLabeller(ctx, nwConfig, testNodeList)
		Expect(err).ToNot(HaveOccurred())
	})
})
***/
