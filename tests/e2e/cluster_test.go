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

package e2e

import (
	"context"
	"strings"

	"github.com/stretchr/testify/assert"
	. "gopkg.in/check.v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ROCm/network-operator/api/v1alpha1"
	"github.com/ROCm/network-operator/tests/e2e/utils"
)

func (s *E2ESuite) TestBasicSkipDriverInstall(c *C) {
	nc := s.getNetworkConfig()
	*nc.Spec.Driver.Enable = false
	logger.Infof("create %s", nc.Name)
	s.createNetworkConfig(nc, c)
	defer s.deleteNetworkConfig(nc, c)
	s.verifyOperandReadiness(c, nc)
}

func (s *E2ESuite) TestDeployment(c *C) {
	_, err := s.nCfgClient.NetworkConfigs(s.ns).Get(s.cfgName, metav1.GetOptions{})
	assert.Errorf(c, err, "config %s exists", s.cfgName)

	nc := s.getNetworkConfig()
	s.createNetworkConfig(nc, c)
	defer s.deleteNetworkConfig(nc, c)
	s.verifyOperandReadiness(c, nc)

	if !s.simEnable {
		s.verifyNodeNICLabel(nc, c)

		err = utils.AddNetworkAttachmentDefinition(s.nadClient)
		assert.NoError(c, err, "create NAD")
		err = utils.DeployPodWithNICResource(context.TODO(), s.k8sClientSet, nil)
		assert.NoError(c, err, "deploy workload")

		// ensure the interface is moved inside the oid
		podNames, err := utils.ListAinicPods(context.TODO(), s.k8sClientSet)
		assert.NoError(c, err, "list NIC pods")
		assert.NotEmpty(c, podNames, "no NIC pods found")
		for _, podName := range podNames {
			// ensure pod phase is Running and all containers are Ready
			pod, gerr := s.k8sClientSet.CoreV1().Pods(v1.NamespaceDefault).Get(context.TODO(), podName, metav1.GetOptions{})
			assert.NoError(c, gerr, "get pod %s", podName)
			assert.Equal(c, v1.PodRunning, pod.Status.Phase, "pod %s not running (phase=%s)", podName, pod.Status.Phase)
			for _, cs := range pod.Status.ContainerStatuses {
				assert.True(c, cs.Ready, "container %s in pod %s not ready (state=%+v)", cs.Name, podName, cs.State)
			}

			// list interfaces
			ifacesOut, err := utils.ExecPodCmd("ls -1 /sys/class/net", "default", podName, "")
			assert.NoError(c, err, "list interfaces in pod %s", podName)
			logger.Infof("pod %s interfaces:\n%s", podName, ifacesOut)

			// look for aditional interfaces that are not loopback or eth0
			var additionalInterfaces []string
			for _, l := range strings.Fields(ifacesOut) {
				if l != "lo" && l != "eth0" {
					additionalInterfaces = append(additionalInterfaces, l)
					logger.Infof("found additional interface %s in pod %s", l, podName)
				}
			}
			assert.True(c, len(additionalInterfaces) == 1, "additional secondary interface not found in the pod %s", podName)
		}

		err = utils.DeleteNetworkAttachmentDefinition(s.nadClient)
		assert.NoError(c, err, "delete NAD")
		err = utils.DeletePodWithNICResource(context.TODO(), s.k8sClientSet)
		assert.NoError(c, err, "delete workload")
	}
}

func (s *E2ESuite) TestDevicePluginNodeLabellerDaemonSetUpgrade(c *C) {
	_, err := s.nCfgClient.NetworkConfigs(s.ns).Get(s.cfgName, metav1.GetOptions{})
	assert.Errorf(c, err, "config %s exists", s.cfgName)

	nc := s.getNetworkConfig()
	nc.Spec.DevicePlugin.UpgradePolicy = &v1alpha1.DaemonSetUpgradeSpec{
		UpgradeStrategy: "RollingUpdate",
		MaxUnavailable:  1,
	}
	s.createNetworkConfig(nc, c)
	defer s.deleteNetworkConfig(nc, c)
	s.verifyOperandReadiness(c, nc)

	// upgrade images
	nc.Spec.DevicePlugin.DevicePluginImage = devicePluginImage2
	nc.Spec.DevicePlugin.NodeLabellerImage = nodeLabellerImage2
	s.patchDevicePluginImage(nc, c)
	s.patchNodeLabellerImage(nc, c)

	s.verifyDevicePluginStatus(nc, c)
	s.verifyNetworkConfigStatus(nc, c)
	s.verifyNodeLabellerStatus(nc, c)
}

func (s *E2ESuite) TestMetricsExporterDaemonSetUpgrade(c *C) {
	if s.simEnable {
		c.Skip("skip metrics exporter upgrade on sim environment")
	}
	_, err := s.nCfgClient.NetworkConfigs(s.ns).Get(s.cfgName, metav1.GetOptions{})
	assert.Errorf(c, err, "config %s exists", s.cfgName)

	nc := s.getNetworkConfig()
	nc.Spec.MetricsExporter.UpgradePolicy = &v1alpha1.DaemonSetUpgradeSpec{
		UpgradeStrategy: "RollingUpdate",
		MaxUnavailable:  2,
	}
	s.createNetworkConfig(nc, c)
	defer s.deleteNetworkConfig(nc, c)
	s.verifyOperandReadiness(c, nc)

	// upgrade exporter image
	nc.Spec.MetricsExporter.Image = exporterImage2
	s.patchMetricsExporterImage(nc, c)

	s.verifyNetworkConfigStatus(nc, c)
	s.verifyMetricsExporterStatus(nc, c)
}

func (s *E2ESuite) TestNodeLabeller(c *C) {
	_, err := s.nCfgClient.NetworkConfigs(s.ns).Get(s.cfgName, metav1.GetOptions{})
	assert.Errorf(c, err, "config %s exists", s.cfgName)
	nc := s.getNetworkConfig()
	s.createNetworkConfig(nc, c)
	s.verifyNodeLabellerStatus(nc, c)

	// Verify that the node labeller is running and has labeled the nodes
	if !s.simEnable {
		logger.Infof("verify node labels on non-sim environment")
		s.verifyNICNLLablelsPresent(nc, c)
	}

	// Delete node labeller and ensure labels are removed
	s.deleteNetworkConfig(nc, c)
	if !s.simEnable {
		logger.Infof("verify node labels are removed on non-sim environment")
		s.verifyNICNLLablelsNotPresent(nc, c)
	}
}

func (s *E2ESuite) TestSecondaryNetworkUpgrade(c *C) {
	_, err := s.nCfgClient.NetworkConfigs(s.ns).Get(s.cfgName, metav1.GetOptions{})
	assert.Errorf(c, err, "config %s exists")

	nc := s.getNetworkConfig()
	s.createNetworkConfig(nc, c)
	defer s.deleteNetworkConfig(nc, c)
	s.verifyOperandReadiness(c, nc)

	// upgrade CNI plugins image
	nc.Spec.SecondaryNetwork.CniPlugins.Image = cniPluginsImage2
	nc.Spec.SecondaryNetwork.CniPlugins.UpgradePolicy = &v1alpha1.DaemonSetUpgradeSpec{
		UpgradeStrategy: "RollingUpdate",
		MaxUnavailable:  1,
	}
	s.patchCNIPluginsImage(nc, c)

	s.verifyNetworkConfigStatus(nc, c)
	s.verifySecondaryNetworkStatus(nc, c)
}

func (s *E2ESuite) verifyOperandReadiness(c *C, nc *v1alpha1.NetworkConfig) {
	// check NFD worker status
	s.verifyNFDWorkerStatus(c)

	// check device plugin status
	s.verifyDevicePluginStatus(nc, c)

	// check node labeller status
	s.verifyNodeLabellerStatus(nc, c)

	// check metrics exporter status
	s.verifyMetricsExporterStatus(nc, c)

	// verify secondary network status
	s.verifySecondaryNetworkStatus(nc, c)

	// verify network config status
	s.verifyNetworkConfigStatus(nc, c)
}
