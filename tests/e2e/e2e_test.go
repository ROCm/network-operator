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
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/common/expfmt"
	. "gopkg.in/check.v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/ROCm/common-infra-operator/pkg/metricsexporter"
	"github.com/ROCm/network-operator/api/v1alpha1"
	"github.com/ROCm/network-operator/internal/kmmmodule"
	"github.com/ROCm/network-operator/tests/e2e/utils"
	"github.com/stretchr/testify/assert"
)

var (
	vnicselector = map[string]string{"feature.node.kubernetes.io/amd-vnic": "true"}
	nicSelector  = map[string]string{"feature.node.kubernetes.io/amd-nic": "true"}
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) {
	TestingT(t)
}

var _ = Suite(&E2ESuite{})

// -----------------------------------------------------------------------------
// Networkconfig generate and CRUD
// -----------------------------------------------------------------------------
func (s *E2ESuite) getNetworkConfig() *v1alpha1.NetworkConfig {
	nc := &v1alpha1.NetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.cfgName,
			Namespace: s.ns,
		},
		Spec: v1alpha1.NetworkConfigSpec{
			Driver: v1alpha1.DriverSpec{
				Enable:  boolPtr(true),
				Image:   "registry.test.pensando.io:5000/amdainic_kmod",
				Version: "1.117.1-a-42",
				ImageRegistryTLS: v1alpha1.RegistryTLS{
					Insecure:              boolPtr(true),
					InsecureSkipTLSVerify: boolPtr(true),
				},
				UpgradePolicy: &v1alpha1.DriverUpgradePolicySpec{
					Enable:              boolPtr(true),
					RebootRequired:      boolPtr(false),
					MaxUnavailableNodes: intstr.FromInt(1),
				},
			},
			MetricsExporter: v1alpha1.MetricsExporterSpec{
				Enable:   boolPtr(true),
				NodePort: 32501,
				Port:     5001,
				Image:    exporterImage,
			},
			DevicePlugin: v1alpha1.DevicePluginSpec{
				DevicePluginImage: devicePluginImage,
				NodeLabellerImage: nodeLabellerImage,
			},
			SecondaryNetwork: v1alpha1.SecondaryNetworkSpec{
				CniPlugins: &v1alpha1.CniPluginsSpec{
					Enable: boolPtr(true),
					Image:  cniPluginsImage,
				},
			},
			Selector: nicSelector,
			CommonConfig: v1alpha1.CommonConfigSpec{
				UtilsContainer: v1alpha1.UtilsContainerSpec{
					Image: generalUtilsImage,
				},
			},
		},
	}

	if s.simEnable {
		nc.Spec.DevicePlugin.EnableNodeLabeller = boolPtr(true)
	}
	return nc
}

func (s *E2ESuite) createNetworkConfig(nc *v1alpha1.NetworkConfig, c *C) {
	_, err := s.nCfgClient.NetworkConfigs(s.ns).Create(nc)
	assert.NoError(c, err, "create NetworkConfig %s", nc.Name)
}

func (s *E2ESuite) deleteNetworkConfig(nc *v1alpha1.NetworkConfig, c *C) {
	logger.Infof("delete NetworkConfig %s", nc.Name)
	_, err := s.nCfgClient.NetworkConfigs(s.ns).Delete(nc.Name)
	assert.NoError(c, err, "delete NetworkConfig %s", nc.Name)

	// wait CR gone
	waitEventually(c, "NetworkConfig removed", timeoutShort, pollInterval, func() (bool, error) {
		_, err := s.nCfgClient.NetworkConfigs(s.ns).Get(nc.Name, metav1.GetOptions{})
		return err != nil, nil
	})
}

func (s *E2ESuite) deleteAllNetworkConfigs(c *C) {
	l, err := s.nCfgClient.NetworkConfigs(s.ns).List(metav1.ListOptions{})
	if err != nil {
		c.Fatalf("list networkconfigs: %v", err)
	}
	if len(l.Items) > 0 {
		for _, cfg := range l.Items {
			logger.Infof("delete NetworkConfig:  %s", cfg.Name)
			if _, err := s.nCfgClient.NetworkConfigs(s.ns).Delete(cfg.Name); err != nil {
				c.Fatalf("delete %s: %v", cfg.Name, err)
			}
		}
		waitEventually(c, "NetworkConfigs removed", timeoutShort, pollInterval, func() (bool, error) {
			lc, err := s.nCfgClient.NetworkConfigs(s.ns).List(metav1.ListOptions{})
			return err == nil && len(lc.Items) == 0, nil
		})
	}
}

func (s *E2ESuite) deleteConfigMap() {
	_ = s.k8sClientSet.CoreV1().ConfigMaps(s.ns).Delete(context.TODO(), s.cfgName, metav1.DeleteOptions{})
}

func (s *E2ESuite) patchDevicePluginImage(nc *v1alpha1.NetworkConfig, c *C) {
	_, err := s.nCfgClient.NetworkConfigs(s.ns).PatchDevicePluginImage(nc)
	assert.NoError(c, err, "patch device plugin image")
}

func (s *E2ESuite) patchNodeLabellerImage(nc *v1alpha1.NetworkConfig, c *C) {
	_, err := s.nCfgClient.NetworkConfigs(s.ns).PatchNodeLabellerImage(nc)
	assert.NoError(c, err, "patch node labeller image")
}

func (s *E2ESuite) patchMetricsExporterImage(nc *v1alpha1.NetworkConfig, c *C) {
	_, err := s.nCfgClient.NetworkConfigs(s.ns).PatchMetricsExporterImage(nc)
	assert.NoError(c, err, "patch metrics exporter image")
}

func (s *E2ESuite) patchCNIPluginsImage(nc *v1alpha1.NetworkConfig, c *C) {
	_, err := s.nCfgClient.NetworkConfigs(s.ns).PatchCNIPluginsImage(nc)
	assert.NoError(c, err, "patch CNI plugins image")
}

func (s *E2ESuite) checkAndUseMockExporterImageForSim(nc *v1alpha1.NetworkConfig) {
	if s.simEnable {
		logger.Infof("sim mode enabled, using mock exporter image: %s", exporterImageMock)
		nc.Spec.MetricsExporter.Image = exporterImageMock
	}
}

// -----------------------------------------------------------------------------
// Component readiness checks
// -----------------------------------------------------------------------------
func (s *E2ESuite) verifyNFDWorkerStatus(c *C) {
	name := utils.NFDWorkerName(s.openshift)
	waitEventually(c, "NFD worker ready", timeoutShort, pollInterval, func() (bool, error) {
		ds, err := s.k8sClientSet.AppsV1().DaemonSets(s.ns).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return ds.Status.DesiredNumberScheduled > 0 &&
			ds.Status.NumberReady == ds.Status.DesiredNumberScheduled, nil
	})
}

func (s *E2ESuite) verifyDevicePluginStatus(nc *v1alpha1.NetworkConfig, c *C) {
	waitEventually(c, "device plugin ready", timeoutShort, pollInterval, func() (bool, error) {
		ds, err := s.k8sClientSet.AppsV1().DaemonSets(s.ns).Get(context.TODO(), utils.DevicePluginName(nc.Name), metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return ds.Status.DesiredNumberScheduled > 0 &&
			ds.Status.NumberReady == ds.Status.DesiredNumberScheduled, nil
	})
}

func (s *E2ESuite) verifySecondaryNetworkStatus(nc *v1alpha1.NetworkConfig, c *C) {
	// check multus status
	waitEventually(c, "Multus ready", timeoutShort, pollInterval, func() (bool, error) {
		ds, err := s.k8sClientSet.AppsV1().DaemonSets(s.ns).Get(context.TODO(), utils.MultusName(), metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return ds.Status.DesiredNumberScheduled > 0 &&
			ds.Status.NumberReady == ds.Status.DesiredNumberScheduled, nil
	})

	// check CNI plugins status
	waitEventually(c, "CNI plugins ready", timeoutShort, pollInterval, func() (bool, error) {
		ds, err := s.k8sClientSet.AppsV1().DaemonSets(s.ns).Get(context.TODO(), utils.CNIPluginsName(nc.Name), metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return ds.Status.DesiredNumberScheduled > 0 &&
			ds.Status.NumberReady == ds.Status.DesiredNumberScheduled, nil
	})
}

func (s *E2ESuite) verifyNodeLabellerStatus(nc *v1alpha1.NetworkConfig, c *C) {
	if nc.Spec.DevicePlugin.EnableNodeLabeller == nil || !*nc.Spec.DevicePlugin.EnableNodeLabeller {
		return
	}
	waitEventually(c, "node labeller ready", timeoutLong, pollInterval, func() (bool, error) {
		ds, err := s.k8sClientSet.AppsV1().DaemonSets(s.ns).Get(context.TODO(), utils.NodeLabellerName(nc.Name), metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return ds.Status.NumberReady > 0 &&
			ds.Status.NumberReady == ds.Status.DesiredNumberScheduled, nil
	})
}

func (s *E2ESuite) verifyMetricsExporterDaemonSetStatus(nc *v1alpha1.NetworkConfig, c *C) {
	waitEventually(c, "metrics exporter DaemonSet ready", timeoutLong, pollInterval, func() (bool, error) {
		dsName := utils.MetricsExporterName(nc.Name)
		ds, err := s.k8sClientSet.AppsV1().DaemonSets(s.ns).Get(context.TODO(), dsName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if ds.Status.NumberReady == 0 || ds.Status.NumberReady != ds.Status.DesiredNumberScheduled {
			return false, fmt.Errorf("metrics exporter DaemonSet %s not ready: %+v", dsName, ds.Status)
		}
		return true, nil
	})
}

func (s *E2ESuite) verifyMetricsExporterServiceStatus(nc *v1alpha1.NetworkConfig, c *C) {
	waitEventually(c, "metrics exporter Service ready", timeoutLong, pollInterval, func() (bool, error) {
		svcName := utils.MetricsExporterName(nc.Name)
		svc, err := s.k8sClientSet.CoreV1().Services(s.ns).Get(context.TODO(), svcName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if len(svc.Spec.Ports) == 0 || svc.Spec.Ports[0].TargetPort != intstr.FromInt32(nc.Spec.MetricsExporter.Port) {
			return false, fmt.Errorf("metrics exporter Service %s not ready: %+v", svcName, svc.Spec)
		}

		ncSvcType := nc.Spec.MetricsExporter.SvcType
		if ncSvcType == "" {
			// Default to ClusterIP
			ncSvcType = v1alpha1.ServiceTypeClusterIP
		}

		// check if the service type matches the expected type
		if ncSvcType != v1alpha1.ServiceType(svc.Spec.Type) {
			return false, fmt.Errorf("service type mismatch, expected %s, got %s", ncSvcType, svc.Spec.Type)
		}

		// Validate port configuration based on service type
		if ncSvcType == v1alpha1.ServiceTypeNodePort {
			if svc.Spec.Ports[0].NodePort != nc.Spec.MetricsExporter.NodePort {
				return false, fmt.Errorf("NodePort service port mismatch, expected %d, got %d", nc.Spec.MetricsExporter.NodePort, svc.Spec.Ports[0].NodePort)
			}
		} else if ncSvcType == v1alpha1.ServiceTypeClusterIP {
			if svc.Spec.Ports[0].Port != nc.Spec.MetricsExporter.Port {
				return false, fmt.Errorf("ClusterIP service port mismatch, expected %d, got %d", nc.Spec.MetricsExporter.Port, svc.Spec.Ports[0].Port)
			}
		}

		return true, nil
	})
}

func (s *E2ESuite) verifyMetricsExporterStatus(nc *v1alpha1.NetworkConfig, c *C) {
	s.verifyMetricsExporterDaemonSetStatus(nc, c)
	s.verifyMetricsExporterServiceStatus(nc, c)
}

func (s *E2ESuite) verifyNetworkConfigStatus(nc *v1alpha1.NetworkConfig, c *C) {
	waitEventually(c, "NetworkConfig status ready", timeoutLong, pollInterval, func() (bool, error) {
		current, err := s.nCfgClient.NetworkConfigs(s.ns).Get(nc.Name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		st := current.Status.DevicePlugin
		return st.NodesMatchingSelectorNumber > 0 &&
			st.NodesMatchingSelectorNumber == st.AvailableNumber &&
			st.DesiredNumber == st.AvailableNumber, nil
	})
}

func (s *E2ESuite) verifyNodeDriverVersionLabel(netCfg *v1alpha1.NetworkConfig, c *C) {
	assert.Eventually(c, func() bool {
		nodes, err := s.k8sClientSet.CoreV1().Nodes().List(context.TODO(),
			metav1.ListOptions{LabelSelector: selectorString(netCfg.Spec.Selector)})
		if err != nil {
			logger.Errorf("failed to get nodes %v", err)
			return false
		}
		allMatched := true
		for _, node := range nodes.Items {
			versionLabelKey, versionLabelValue := kmmmodule.GetVersionLabelKV(netCfg)
			if versionLabelValue == "" {
				versionLabelValue = s.defaultDriverVersion
			}
			if ver, ok := node.Labels[versionLabelKey]; !ok {
				logger.Errorf("failed to find driver version label %+v on node %+v", versionLabelKey, node.Name)
				allMatched = false
			} else if ver != versionLabelValue {
				logger.Errorf("mismatched driver version label, node resource has %+v but expect %+v", ver, versionLabelValue)
				allMatched = false
			}
		}
		return allMatched
	}, 5*time.Minute, 5*time.Second)
}

func (s *E2ESuite) verifyNodeModuleStatus(netCfg *v1alpha1.NetworkConfig, expectedNodeStatus v1alpha1.UpgradeState, c *C) {
	assert.Eventually(c, func() bool {
		nc, err := s.nCfgClient.NetworkConfigs(s.ns).Get(netCfg.Name, metav1.GetOptions{})
		if err != nil {
			logger.Errorf("Failed to get network config %v", err)
			return false
		}

		// Check if all NodeModuleStatus entries are in UpgradeStateComplete
		for nodeName, moduleStatus := range nc.Status.NodeModuleStatus {
			if moduleStatus.Status != expectedNodeStatus {
				logger.Infof("Upgrade status not matched for node %s: expected %s, got %s", nodeName, expectedNodeStatus, moduleStatus.Status)
				return false
			}
			if expectedNodeStatus == v1alpha1.UpgradeStateComplete || expectedNodeStatus == v1alpha1.UpgradeStateInstallComplete {
				if !strings.HasSuffix(moduleStatus.ContainerImage, nc.Spec.Driver.Version) {
					logger.Infof("Upgrade/Install not complete for node %s with state container image %s while NetworkConfig specified version %s",
						nodeName, moduleStatus.ContainerImage, nc.Spec.Driver.Version)
					return false
				}
			}
		}

		logger.Infof("All nodes are in expected state %s", expectedNodeStatus)
		return true

	}, 30*time.Minute, 5*time.Second)
}

// -----------------------------------------------------------------------------
// Label verification
// -----------------------------------------------------------------------------
func (s *E2ESuite) verifyNodeNICLabel(nc *v1alpha1.NetworkConfig, c *C) {
	waitEventually(c, "NIC capacity/allocatable labels", timeoutShort, pollInterval, func() (bool, error) {
		nodes, err := s.k8sClientSet.CoreV1().Nodes().List(context.TODO(),
			metav1.ListOptions{LabelSelector: selectorString(nc.Spec.Selector)})
		if err != nil || len(nodes.Items) == 0 {
			return false, nil
		}
		for _, n := range nodes.Items {
			if !utils.CheckNicLabel(n.Status.Capacity) || !utils.CheckNicLabel(n.Status.Allocatable) {
				return false, nil
			}
		}
		return true, nil
	})
}

func (s *E2ESuite) verifyNICNLLablelsPresent(nc *v1alpha1.NetworkConfig, c *C) {
	labelPatterns := map[string]string{
		`amd.com/nic\..*count`:            `^\d+$`,
		`amd.com/nic\..*product-name`:     `^.+$`,
		`amd.com/nic\..*firmware-version`: `^.+$`,
		`amd.com/nic\..*port-count`:       `^\d+$`,
		`amd.com/nic\..*port[0-9]*-speed`: `^\d+[A-Za-z]+$`,
		`amd.com/nic.driver-version`:      `^.+$`,
		`amd.com/nic.driver-name`:         `^.+$`,
	}
	compiled := make(map[*regexp.Regexp]*regexp.Regexp, len(labelPatterns))
	for k, v := range labelPatterns {
		compiled[regexp.MustCompile(k)] = regexp.MustCompile(v)
	}

	waitEventually(c, "NIC node-labeller labels present", timeoutShort, pollInterval, func() (bool, error) {
		nodes, err := s.k8sClientSet.CoreV1().Nodes().List(context.TODO(),
			metav1.ListOptions{LabelSelector: selectorString(nc.Spec.Selector)})
		if err != nil || len(nodes.Items) == 0 {
			return false, nil
		}
		for _, n := range nodes.Items {
			for kRe, vRe := range compiled {
				found := false
				for lk, lv := range n.Labels {
					if kRe.MatchString(lk) {
						found = true
						if !vRe.MatchString(lv) {
							return false, nil
						}
						break
					}
				}
				if !found {
					return false, nil
				}
			}
		}
		return true, nil
	})
}

func (s *E2ESuite) verifyNICNLLablelsNotPresent(nc *v1alpha1.NetworkConfig, c *C) {
	waitEventually(c, "NIC node-labeller labels not found", timeoutShort, pollInterval, func() (bool, error) {
		nodes, err := s.k8sClientSet.CoreV1().Nodes().List(context.TODO(),
			metav1.ListOptions{LabelSelector: selectorString(nc.Spec.Selector)})
		if err != nil {
			return false, nil
		}
		for _, n := range nodes.Items {
			for k := range n.Labels {
				if strings.HasPrefix(k, "amd.com/nic") {
					return false, nil
				}
			}
		}
		return true, nil
	})
}

// -----------------------------------------------------------------------------
// Ensure all pods in a daemon set honor the selector
// -----------------------------------------------------------------------------
func (s *E2ESuite) verifySelectorFunctionalityForDaemonSet(appLabel map[string]string, nodeLabel map[string]string, c *C) {
	pods, _ := s.k8sClientSet.CoreV1().Pods(s.ns).List(context.TODO(), metav1.ListOptions{
		LabelSelector: kmmmodule.MapToLabelSelector(appLabel),
	})

	nodes, _ := s.k8sClientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: kmmmodule.MapToLabelSelector(nodeLabel),
	})
	assert.True(c, len(nodes.Items) > 0, "no nodes matching selector", len(nodes.Items))
	assert.True(c, len(pods.Items) > 0, "no pods found for label", len(pods.Items))
	assert.Equal(c, len(nodes.Items), len(pods.Items), "number of pods should match number of nodes with selector")

	// Create node name map for quick search
	nodeNameMap := make(map[string]bool, len(nodes.Items))
	for _, node := range nodes.Items {
		nodeNameMap[node.Name] = true
	}

	for _, pod := range pods.Items {
		found := nodeNameMap[pod.Spec.NodeName]
		assert.True(c, found, "pod %s is not scheduled on a node matching the selector", pod.Name)
	}
	logger.Infof("all %d pods scheduled on nodes matching selector", len(pods.Items))
}

func (s *E2ESuite) verifyNoMetricsExporter(netCfg *v1alpha1.NetworkConfig, c *C) {
	ns := netCfg.Namespace
	waitEventually(c, "NIC metrics exporter pods and service not present", timeoutShort, pollInterval, func() (bool, error) {
		if _, err := s.k8sClientSet.AppsV1().DaemonSets(ns).Get(context.TODO(), netCfg.Name+"-"+metricsexporter.ExporterName,
			metav1.GetOptions{}); err == nil {
			return false, fmt.Errorf("metrics exporter exists: %+v %v", netCfg, err)
		}

		if _, err := s.k8sClientSet.CoreV1().Services(ns).Get(context.TODO(),
			netCfg.Name+"-"+metricsexporter.ExporterName, metav1.GetOptions{}); err == nil {
			logger.Warnf("metrics service exists: %+v %v", netCfg, err)
			return false, fmt.Errorf("metrics service exists: %+v %v", netCfg, err)
		}

		return true, nil
	})
}

func (s *E2ESuite) getMetricsQueryClientPod(namespace string, c *C) (*v1.Pod, error) {
	return s.k8sClientSet.CoreV1().Pods(namespace).Get(context.TODO(), queryClientPodName, metav1.GetOptions{})
}

func (s *E2ESuite) metricsQueryClientPod(namespace string, c *C) *v1.Pod {
	// Delete existing pod if already present
	if _, err := s.getMetricsQueryClientPod(namespace, c); err == nil {
		s.cleanupQueryClientPod(namespace, c)
	}

	// Create a new utils pod
	clientPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      queryClientPodName,
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyNever,
			Containers: []v1.Container{
				{
					Name:    "utils-container",
					Image:   metricsQueryClientImage,
					Command: []string{"sleep", "3600"},
				},
			},
		},
	}

	// Create the client pod
	_, err := s.k8sClientSet.CoreV1().Pods(namespace).Create(context.TODO(), clientPod, metav1.CreateOptions{})
	if err != nil {
		c.Fatalf("failed to create metrics query client pod: %v", err)
	}

	return clientPod
}

func (s *E2ESuite) cleanupQueryClientPod(namespace string, c *C) {
	pod, err := s.getMetricsQueryClientPod(namespace, c)
	// Return early if the client pod is not present
	if err != nil {
		return
	}

	if err := s.k8sClientSet.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{}); err != nil {
		c.Fatalf("failed to delete metrics query client pod: %v", err)
	}

	// wait for pod to be completely removed
	waitEventually(c, fmt.Sprintf("pod %s deleted", pod.Name), timeoutShort, pollInterval, func() (bool, error) {
		_, err := s.getMetricsQueryClientPod(namespace, c)
		return err != nil, nil
	})
}

func (s *E2ESuite) getExporterServiceEndpoint(netCfg *v1alpha1.NetworkConfig) (string, error) {
	serviceName := netCfg.Name + "-" + metricsexporter.ExporterName
	svc, err := s.k8sClientSet.CoreV1().Services(netCfg.Namespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get service %s: %w", serviceName, err)
	}

	svcType := svc.Spec.Type
	switch svcType {
	case v1.ServiceTypeClusterIP:
		if svc.Spec.ClusterIP == "" || svc.Spec.ClusterIP == "None" {
			return "", fmt.Errorf("service %s has no cluster IP", serviceName)
		}
		port := netCfg.Spec.MetricsExporter.Port
		if port == 0 && len(svc.Spec.Ports) > 0 {
			port = svc.Spec.Ports[0].Port
		}
		return fmt.Sprintf("%s:%d", svc.Spec.ClusterIP, port), nil
	case v1.ServiceTypeNodePort:
		if len(svc.Spec.Ports) == 0 {
			return "", fmt.Errorf("service %s has no ports", serviceName)
		}
		nodePort := svc.Spec.Ports[0].NodePort
		// Get any node IP for NodePort access
		nodes, err := s.k8sClientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
			LabelSelector: selectorString(netCfg.Spec.Selector),
		})
		if err != nil {
			return "", fmt.Errorf("failed to get nodes: %w", err)
		}
		if len(nodes.Items) == 0 {
			return "", fmt.Errorf("no nodes found")
		}
		for _, addr := range nodes.Items[0].Status.Addresses {
			if addr.Type == v1.NodeInternalIP {
				return fmt.Sprintf("%s:%d", addr.Address, nodePort), nil
			}
		}
		return "", fmt.Errorf("no internal IP found for node %s", nodes.Items[0].Name)
	}

	return "", fmt.Errorf("unsupported service type: %v", svcType)
}

func (s *E2ESuite) verifyMetricsPresent(
	endpoint string,
	fieldsAndLabels map[string][]string,
	token string,
	secure bool,
	verifyTLS bool,
	caCertPath string,
	clientCertPath string,
	clientKeyPath string,
	clientPod *v1.Pod,
	c *C,
) {
	if len(fieldsAndLabels) == 0 {
		fieldsAndLabels = map[string][]string{
			"nic_total":                        {"hostname"},
			"nic_port_stats_frames_rx_all":     {"hostname"},
			"nic_port_stats_frames_tx_all":     {"hostname"},
			"nic_port_stats_frames_rx_bad_all": {"hostname"},
		}
	}

	waitEventually(c, "NIC metrics present", timeoutShort, pollInterval, func() (bool, error) {
		proto := "http"
		if secure {
			proto = "https"
		}
		metricsOutput, err := utils.DoCurl(
			fmt.Sprintf("%s://%s/metrics", proto, endpoint),
			token,
			verifyTLS,
			caCertPath,
			clientCertPath,
			clientKeyPath,
			clientPod,
			true,
			true,
		)
		if err != nil {
			return false, fmt.Errorf("failed to curl metrics: %w", err)
		}

		if metricsOutput == "" {
			return false, fmt.Errorf("received empty metrics response")
		}

		p := expfmt.TextParser{}
		m, err := p.TextToMetricFamilies(strings.NewReader(metricsOutput))
		if err != nil {
			return false, fmt.Errorf("failed to parse metrics data: %v", err)
		}

		for field, labels := range fieldsAndLabels {
			k, ok := m[field]
			if !ok || k == nil {
				return false, fmt.Errorf("field %s not found", field)
			}

			metricsLabels := map[string]string{}
			for _, km := range k.Metric {
				for _, lp := range km.GetLabel() {
					metricsLabels[*lp.Name] = *lp.Value
				}
			}

			logger.Infof("found field %v labels %v", field, metricsLabels)

			for _, l := range labels {
				_, ok := metricsLabels[l]
				if !ok {
					return false, fmt.Errorf("missing label %v for field %v", l, field)
				}
			}
		}
		return true, nil
	})
}

func (s *E2ESuite) verifyMetricsNotPresent(
	c *C,
	endpoint string,
	exemptedMetrics []string,
	token string,
	secure bool,
	verifyTLS bool,
	caCertPath string,
	clientCertPath string,
	clientKeyPath string,
	clientPod *v1.Pod,
) {
	waitEventually(c, "Only exempted NIC metrics present", timeoutShort, pollInterval, func() (bool, error) {
		proto := "http"
		if secure {
			proto = "https"
		}
		metricsOutput, err := utils.DoCurl(
			fmt.Sprintf("%s://%s/metrics", proto, endpoint),
			token,
			verifyTLS,
			caCertPath,
			clientCertPath,
			clientKeyPath,
			clientPod,
			true,
			true,
		)
		if err != nil {
			return false, fmt.Errorf("failed to curl metrics: %w", err)
		}

		p := expfmt.TextParser{}
		m, err := p.TextToMetricFamilies(strings.NewReader(metricsOutput))
		if err != nil {
			return false, fmt.Errorf("failed to parse metrics data: %v", err)
		}

		// Create a map for fast lookup of exempted metrics
		exemptedMap := make(map[string]bool)
		for _, metric := range exemptedMetrics {
			exemptedMap[metric] = true
		}

		// Check all present metrics
		for metricName := range m {
			if strings.HasPrefix(metricName, "prom") {
				continue // Skip prometheus internal metrics
			}

			// If metric is not in exempted list, fail
			if !exemptedMap[metricName] {
				return false, fmt.Errorf("non-exempted metric found: %s", metricName)
			}
		}

		logger.Info("No unexpected metric found")
		return true, nil
	})
}

func (s *E2ESuite) verifyMetricsPresentViaSvc(
	netCfg *v1alpha1.NetworkConfig,
	svcType v1alpha1.ServiceType,
	fieldsAndLabels map[string][]string,
	token string,
	secure bool,
	verifyTLS bool,
	caCertPath string,
	clientCertPath string,
	clientKeyPath string,
	clientPod *v1.Pod,
	c *C,
) {
	// Set service type
	logger.Infof("Setting metrics exporter svc type for network config %s to %s", netCfg.Name, svcType)
	netCfg.Spec.MetricsExporter.SvcType = svcType
	netCfg, err := s.nCfgClient.NetworkConfigs(s.ns).PatchMetricsExporterServiceType(netCfg)
	assert.NoError(c, err, "failed to patch %v", netCfg.Name)
	s.verifyMetricsExporterServiceStatus(netCfg, c)

	// Get endpoint and verify metrics
	endpoint, err := s.getExporterServiceEndpoint(netCfg)
	assert.NoError(c, err, "failed to get exporter service endpoint")
	s.verifyMetricsPresent(
		endpoint,
		fieldsAndLabels,
		token,
		secure,
		verifyTLS,
		caCertPath,
		clientCertPath,
		clientKeyPath,
		clientPod,
		c,
	)
}

func (s *E2ESuite) verifyMetricsPresentViaSvcSimple(
	netCfg *v1alpha1.NetworkConfig,
	svcType v1alpha1.ServiceType,
	fieldsAndLabels map[string][]string,
	clientPod *v1.Pod,
	c *C,
) {
	s.verifyMetricsPresentViaSvc(
		netCfg,
		svcType,
		fieldsAndLabels,
		"",
		false,
		false,
		"",
		"",
		"",
		clientPod,
		c,
	)
}

func (s *E2ESuite) verifyMetricsNotPresentViaSvc(
	netCfg *v1alpha1.NetworkConfig,
	svcType v1alpha1.ServiceType,
	exemptedMetrics []string,
	token string,
	secure bool,
	verifyTLS bool,
	caCertPath string,
	clientCertPath string,
	clientKeyPath string,
	clientPod *v1.Pod,
	c *C,
) {
	// Set service type
	logger.Infof("Setting metrics exporter svc type for network config %s to %s", netCfg.Name, svcType)
	netCfg.Spec.MetricsExporter.SvcType = svcType
	netCfg, err := s.nCfgClient.NetworkConfigs(s.ns).PatchMetricsExporterServiceType(netCfg)
	assert.NoError(c, err, "failed to patch %v", netCfg.Name)
	s.verifyMetricsExporterServiceStatus(netCfg, c)

	// Get endpoint and verify metrics
	endpoint, err := s.getExporterServiceEndpoint(netCfg)
	assert.NoError(c, err, "failed to get exporter service endpoint")
	s.verifyMetricsNotPresent(
		c,
		endpoint,
		exemptedMetrics,
		token,
		secure,
		verifyTLS,
		caCertPath,
		clientCertPath,
		clientKeyPath,
		clientPod,
	)
}

func (s *E2ESuite) verifyMetricsNotPresentViaSvcSimple(
	netCfg *v1alpha1.NetworkConfig,
	svcType v1alpha1.ServiceType,
	exemptedMetrics []string,
	clientPod *v1.Pod,
	c *C,
) {
	s.verifyMetricsNotPresentViaSvc(
		netCfg,
		svcType,
		exemptedMetrics,
		"",
		false,
		false,
		"",
		"",
		"",
		clientPod,
		c,
	)
}

func (s *E2ESuite) createExporterConfigmap(name string, namespace string, fields []string, labels []string, c *C) error {
	cfgData, err := json.Marshal(struct {
		NICConfig struct {
			Fields []string
			Labels []string
		}
	}{
		NICConfig: struct {
			Fields []string
			Labels []string
		}{
			Fields: fields,
			Labels: labels,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal config data: %w", err)
	}
	mcfgMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{
			"config.json": string(cfgData),
		},
	}
	_, err = s.k8sClientSet.CoreV1().ConfigMaps(namespace).Create(context.TODO(), mcfgMap, metav1.CreateOptions{})
	return err
}

func (s *E2ESuite) deleteExporterConfigmap(name string, namespace string, c *C) {
	err := s.k8sClientSet.CoreV1().ConfigMaps(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	assert.NoError(c, err, "failed to delete configmap %v", name)
}

// setupKubeRbacCerts generates a CA, server TLS cert/key (with SANs), and optionally a client cert/key.
func (s *E2ESuite) setupKubeRbacCerts(c *C, includeClient bool) (
	caCertPEM, serverCertPEM, serverKeyPEM, clientCertPEM, clientKeyPEM []byte,
	err error,
) {
	// Generate CA
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return
	}
	caTmpl := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{Organization: []string{"My CA"}},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, &caTmpl, &caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		return
	}
	caBuf := &bytes.Buffer{}
	err = pem.Encode(caBuf, &pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	if err != nil {
		return
	}

	// Generate Server cert/key with SANs
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return
	}
	nodeIPs, err := utils.GetNodeIPs(s.k8sClientSet)
	assert.NoError(c, err, "failed to get node IPs for SANs")
	ips := make([]net.IP, 0, len(nodeIPs))
	for _, ip := range nodeIPs {
		ips = append(ips, net.ParseIP(ip))
	}
	serverTmpl := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{Organization: []string{"My TLS"}, CommonName: "metrics-server"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  ips,
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, &serverTmpl, &caTmpl, &serverKey.PublicKey, caKey)
	if err != nil {
		return
	}
	serverBuf := &bytes.Buffer{}
	err = pem.Encode(serverBuf, &pem.Block{Type: "CERTIFICATE", Bytes: serverDER})
	if err != nil {
		return
	}
	err = pem.Encode(serverBuf, &pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	if err != nil {
		return
	}
	keyBuf := &bytes.Buffer{}
	privBytes, err := x509.MarshalPKCS8PrivateKey(serverKey)
	if err != nil {
		return
	}
	err = pem.Encode(keyBuf, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
	if err != nil {
		return
	}

	caCertPEM = caBuf.Bytes()
	serverCertPEM = serverBuf.Bytes()
	serverKeyPEM = keyBuf.Bytes()

	if includeClient {
		// Generate Client cert/key
		var clientKey *rsa.PrivateKey
		var clientDER, privCliBytes []byte
		clientKey, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return
		}
		clientTmpl := x509.Certificate{
			SerialNumber: big.NewInt(3),
			Subject:      pkix.Name{Organization: []string{"My Client"}, CommonName: "metrics-reader"},
			NotBefore:    time.Now(),
			NotAfter:     time.Now().Add(365 * 24 * time.Hour),
			KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		}
		clientDER, err = x509.CreateCertificate(rand.Reader, &clientTmpl, &caTmpl, &clientKey.PublicKey, caKey)
		if err != nil {
			return
		}
		cliCertBuf := &bytes.Buffer{}
		err = pem.Encode(cliCertBuf, &pem.Block{Type: "CERTIFICATE", Bytes: clientDER})
		if err != nil {
			return
		}
		cliKeyBuf := &bytes.Buffer{}
		privCliBytes, err = x509.MarshalPKCS8PrivateKey(clientKey)
		if err != nil {
			return
		}
		err = pem.Encode(cliKeyBuf, &pem.Block{Type: "PRIVATE KEY", Bytes: privCliBytes})
		if err != nil {
			return
		}
		clientCertPEM = cliCertBuf.Bytes()
		clientKeyPEM = cliKeyBuf.Bytes()
	}
	return
}

// -----------------------------------------------------------------------------
// Driver module verification utilities
// -----------------------------------------------------------------------------

func (s *E2ESuite) createDriverCheckUtilsDS(namespace string, selector map[string]string, c *C) *appsv1.DaemonSet {
	// Clean up any existing DaemonSet first
	s.cleanupDriverCheckUtilsDS(namespace, c)

	// Create a DaemonSet with the node selector
	dsName := "driver-check-utils"
	hostPathDirectory := v1.HostPathDirectory
	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dsName,
			Namespace: namespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "driver-check-utils",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "driver-check-utils",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:    "utils-container",
							Image:   generalUtilsImage,
							Command: []string{"sleep", "3600"},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "etc-volume",
									MountPath: "/host/etc",
								},
							},
						},
					},
					NodeSelector: selector,
					Volumes: []v1.Volume{
						{
							Name: "etc-volume",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/etc",
									Type: &hostPathDirectory,
								},
							},
						},
					},
				},
			},
		},
	}

	// Create the DaemonSet
	createdDS, err := s.k8sClientSet.AppsV1().DaemonSets(namespace).Create(context.TODO(), daemonSet, metav1.CreateOptions{})
	if err != nil {
		c.Fatalf("failed to create driver check utils DaemonSet: %v", err)
	}

	// Wait for DaemonSet to be ready
	waitEventually(c, "driver check utils DaemonSet ready", timeoutShort, pollInterval, func() (bool, error) {
		ds, err := s.k8sClientSet.AppsV1().DaemonSets(namespace).Get(context.TODO(), dsName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return ds.Status.DesiredNumberScheduled > 0 &&
			ds.Status.NumberReady == ds.Status.DesiredNumberScheduled, nil
	})

	logger.Infof("Created DaemonSet %s in namespace %s", dsName, namespace)
	return createdDS
}

func (s *E2ESuite) cleanupDriverCheckUtilsDS(namespace string, c *C) {
	dsName := "driver-check-utils"

	// Delete the DaemonSet if it exists
	err := s.k8sClientSet.AppsV1().DaemonSets(namespace).Delete(context.TODO(), dsName, metav1.DeleteOptions{})
	if err != nil {
		// Ignore not found errors
		return
	}

	// Wait for all pods to be completely removed
	waitEventually(c, "all driver check utils pods deleted", timeoutShort, pollInterval, func() (bool, error) {
		remainingPods, err := s.k8sClientSet.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: "app=driver-check-utils",
		})
		if err != nil {
			return false, err
		}
		return len(remainingPods.Items) == 0, nil
	})
}

// -----------------------------------------------------------------------------
// Driver module verification
// -----------------------------------------------------------------------------

func (s *E2ESuite) verifyIonicModuleVersion(netCfg *v1alpha1.NetworkConfig, ds *appsv1.DaemonSet, c *C) {
	expectedIonicVersion := ""
	switch netCfg.Spec.Driver.Version {
	case "", "1.117.1-a-42":
		expectedIonicVersion = "25.08.4.003"
	case "1.117.1-a-63":
		expectedIonicVersion = "25.08.4.004"
	default:
		c.Errorf("unknown driver version")
	}

	waitEventually(c, "ionic module version verified on all nodes", timeoutLong, pollInterval, func() (bool, error) {
		// First, verify the DaemonSet is ready
		currentDS, err := s.k8sClientSet.AppsV1().DaemonSets(ds.Namespace).Get(context.TODO(), ds.Name, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to get DaemonSet: %w", err)
		}

		if currentDS.Status.DesiredNumberScheduled == 0 || currentDS.Status.NumberReady != currentDS.Status.DesiredNumberScheduled {
			return false, fmt.Errorf("DaemonSet not ready: desired=%d, ready=%d", currentDS.Status.DesiredNumberScheduled, currentDS.Status.NumberReady)
		}

		// Get the pods created by the DaemonSet
		podList, err := s.k8sClientSet.CoreV1().Pods(ds.Namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: "app=driver-check-utils",
		})
		if err != nil {
			return false, fmt.Errorf("failed to list pods: %w", err)
		}

		if len(podList.Items) == 0 {
			return false, fmt.Errorf("no pods found for DaemonSet")
		}

		allVerified := true
		for _, pod := range podList.Items {
			// Ensure pod is running
			if pod.Status.Phase != v1.PodRunning {
				logger.Warnf("pod %s not running, current phase: %s", pod.Name, pod.Status.Phase)
				allVerified = false
				continue
			}

			// Check if the ionic version file exists
			checkCmd := "test -f /sys/module/ionic/version"
			_, err := utils.ExecPodCmd(checkCmd, netCfg.Namespace, pod.Name, "utils-container")
			if err != nil {
				logger.Errorf("ionic version file not present on node %s: %v", pod.Spec.NodeName, err)
				allVerified = false
				continue
			}

			cmd := "cat /sys/module/ionic/version"
			output, err := utils.ExecPodCmd(cmd, netCfg.Namespace, pod.Name, "utils-container")
			if err != nil {
				logger.Warnf("failed to find ionic version from pod %s on node %s: %v", pod.Name, pod.Spec.NodeName, err)
				allVerified = false
				continue
			}
			gotIonicVersion := strings.TrimSpace(output)

			if gotIonicVersion != expectedIonicVersion {
				logger.Warnf("ionic module version mismatch on node %s: expected %s, got %s", pod.Spec.NodeName, expectedIonicVersion, gotIonicVersion)
				allVerified = false
			} else {
				logger.Infof("ionic module version verified on node %s: %s", pod.Spec.NodeName, gotIonicVersion)
			}
		}
		return allVerified, nil
	})
}

func (s *E2ESuite) verifyIonicModuleBlacklistPresent(netCfg *v1alpha1.NetworkConfig, expect bool, ds *appsv1.DaemonSet, c *C) {
	// Execute command to check ionic module presence on all provided pods
	cmd := "ls /host/etc/modprobe.d/blacklist-ionic-netop.conf && grep 'blacklist ionic' /host/etc/modprobe.d/blacklist-ionic-netop.conf"

	waitEventually(c, "ionic module blacklist not present on all nodes", timeoutShort, pollInterval, func() (bool, error) {
		// Get the pods created by the DaemonSet
		podList, err := s.k8sClientSet.CoreV1().Pods(ds.Namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: "app=driver-check-utils",
		})
		if err != nil {
			return false, fmt.Errorf("failed to list pods: %w", err)
		}

		if len(podList.Items) == 0 {
			return false, fmt.Errorf("no pods found for DaemonSet")
		}

		allVerified := true
		for _, pod := range podList.Items {
			// Ensure pod is running
			if pod.Status.Phase != v1.PodRunning {
				logger.Warnf("pod %s not running, current phase: %s", pod.Name, pod.Status.Phase)
				allVerified = false
				continue
			}
			_, err := utils.ExecPodCmd(cmd, netCfg.Namespace, pod.Name, "utils-container")
			if err != nil {
				if expect {
					// if blacklist expected but not found
					logger.Warnf("ionic module blacklist file not present on node %s but existence expectation is %v", pod.Spec.NodeName, expect)
					allVerified = false
				}
			} else {
				if !expect {
					// if blacklist not expected but found
					logger.Warnf("ionic module blacklist file present on node %s but existence expectation is %v", pod.Spec.NodeName, expect)
					allVerified = false
				}
			}
		}
		return allVerified, nil
	})
}

func (s *E2ESuite) verifyIonicModuleNotPresent(netCfg *v1alpha1.NetworkConfig, ds *appsv1.DaemonSet, c *C) {
	// Execute command to check ionic module presence on all provided pods
	cmd := "ls /sys/module/ionic"

	waitEventually(c, "ionic module not present on all nodes", timeoutShort, pollInterval, func() (bool, error) {
		// Get the pods created by the DaemonSet
		podList, err := s.k8sClientSet.CoreV1().Pods(ds.Namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: "app=driver-check-utils",
		})
		if err != nil {
			return false, fmt.Errorf("failed to list pods: %w", err)
		}

		if len(podList.Items) == 0 {
			return false, fmt.Errorf("no pods found for DaemonSet")
		}

		allVerified := true
		for _, pod := range podList.Items {
			// Ensure pod is running
			if pod.Status.Phase != v1.PodRunning {
				logger.Warnf("pod %s not running, current phase: %s", pod.Name, pod.Status.Phase)
				allVerified = false
				continue
			}

			_, err := utils.ExecPodCmd(cmd, netCfg.Namespace, pod.Name, "utils-container")
			if err == nil {
				logger.Warnf("ionic module still present on node %s", pod.Spec.NodeName)
				allVerified = false
			} else {
				logger.Infof("ionic module not present on node %s", pod.Spec.NodeName)
			}
		}
		return allVerified, nil
	})
}
