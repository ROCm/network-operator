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
	"fmt"
	"os"
	"time"

	"github.com/ROCm/common-infra-operator/pkg/metricsexporter"
	"github.com/ROCm/network-operator/api/v1alpha1"
	"github.com/ROCm/network-operator/internal/conditions"
	"github.com/ROCm/network-operator/internal/kmmmodule"
	"github.com/ROCm/network-operator/tests/e2e/utils"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	. "gopkg.in/check.v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const (
	queryClientPodName   = "metrics-query-pod-e2e"
	serviceMonitorCRDURL = "https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.81.0/example/prometheus-operator-crd/monitoring.coreos.com_servicemonitors.yaml"
)

// TestExporterBasic tests basic functionality of the metrics exporter
func (s *E2ESuite) TestExporterBasic(c *C) {
	_, err := s.nCfgClient.NetworkConfigs(s.ns).Get(s.cfgName, metav1.GetOptions{})
	assert.Errorf(c, err, fmt.Sprintf("config %v exists", s.cfgName))

	exporterEnable := false
	netCfg := s.getNetworkConfig()
	s.checkAndUseMockExporterImageForSim(netCfg)
	netCfg.Spec.MetricsExporter.Enable = &exporterEnable
	logger.Infof("create network-config %+v", netCfg.Spec.MetricsExporter)
	s.createNetworkConfig(netCfg, c)
	s.verifyNoMetricsExporter(netCfg, c)

	exporterEnable = true
	netCfg.Spec.MetricsExporter.Enable = &exporterEnable
	netCfg, err = s.nCfgClient.NetworkConfigs(s.ns).PatchMetricsExporterEnablement(netCfg)
	assert.NoError(c, err, fmt.Sprintf("failed to patch %v", netCfg.Name))
	s.verifyMetricsExporterStatus(netCfg, c)

	// Create client pod and ensure deferred cleanup
	clientPod := s.metricsQueryClientPod(netCfg.Namespace, c)
	defer s.cleanupQueryClientPod(netCfg.Namespace, c)

	// Verify metrics are present using client pod
	s.verifyMetricsPresentViaSvcSimple(netCfg, v1alpha1.ServiceTypeClusterIP, nil, clientPod, c)
	s.verifyMetricsPresentViaSvcSimple(netCfg, v1alpha1.ServiceTypeNodePort, nil, clientPod, c)

	// Change ports and verify
	netCfg.Spec.MetricsExporter.Port = 6000
	netCfg.Spec.MetricsExporter.SvcType = v1alpha1.ServiceTypeClusterIP
	netCfg, err = s.nCfgClient.NetworkConfigs(s.ns).PatchMetricsExporter(netCfg)
	assert.NoError(c, err, fmt.Sprintf("failed to patch %v", netCfg.Name))
	s.verifyMetricsExporterServiceStatus(netCfg, c)
	netCfg.Spec.MetricsExporter.NodePort = 32601
	netCfg.Spec.MetricsExporter.SvcType = v1alpha1.ServiceTypeNodePort
	netCfg, err = s.nCfgClient.NetworkConfigs(s.ns).PatchMetricsExporterServiceType(netCfg)
	assert.NoError(c, err, fmt.Sprintf("failed to patch %v", netCfg.Name))
	s.verifyMetricsExporterServiceStatus(netCfg, c)

	// Explicitly specify node for exporter and verify scheduling
	nodes, _ := s.k8sClientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: kmmmodule.MapToLabelSelector(netCfg.Spec.Selector),
	})
	assert.True(c, len(nodes.Items) > 0, "no nodes matching selector", len(nodes.Items))
	logger.Infof("selecting selector to %s=%s", "kubernetes.io/hostname", nodes.Items[0].Name)
	netCfg.Spec.MetricsExporter.Selector = map[string]string{"kubernetes.io/hostname": nodes.Items[0].Name}
	netCfg, err = s.nCfgClient.NetworkConfigs(s.ns).PatchMetricsExporter(netCfg)
	assert.NoError(c, err, fmt.Sprintf("failed to patch %v", netCfg.Name))
	s.verifyMetricsExporterStatus(netCfg, c)
	s.verifySelectorFunctionalityForDaemonSet(
		map[string]string{"app.kubernetes.io/name": metricsexporter.ExporterName},
		netCfg.Spec.MetricsExporter.Selector,
		c,
	)
}

// TestExporterConfigmap tests the functionality of metrics exporter with a configmap
func (s *E2ESuite) TestExporterConfigmap(c *C) {
	_, err := s.nCfgClient.NetworkConfigs(s.ns).Get(s.cfgName, metav1.GetOptions{})
	assert.Errorf(c, err, fmt.Sprintf("config %v exists", s.cfgName))

	netCfg := s.getNetworkConfig()
	s.checkAndUseMockExporterImageForSim(netCfg)
	logger.Infof("create network-config %+v", netCfg.Spec.MetricsExporter)
	s.createNetworkConfig(netCfg, c)
	s.verifyMetricsExporterStatus(netCfg, c)

	// Create ConfigMap
	fieldsAndLabels := map[string][]string{
		"nic_port_stats_frames_tx_ok": {"port_name", "pcie_bus_id"},
		"nic_port_stats_frames_rx_ok": {"port_name", "pcie_bus_id"},
	}
	cmFields := []string{"nic_port_stats_frames_tx_ok", "nic_port_stats_frames_rx_ok"}
	cmLabels := []string{"port_name", "pcie_bus_id"}
	cfgMapName := netCfg.Name + "-exporter-config"
	err = s.createExporterConfigmap(cfgMapName, netCfg.Namespace, cmFields, cmLabels, c)
	assert.NoError(c, err, "failed to create configmap %v", cfgMapName)
	defer s.deleteExporterConfigmap(cfgMapName, netCfg.Namespace, c)

	// Patch network config with configmap name
	netCfg.Spec.MetricsExporter.Config.Name = cfgMapName
	netCfg, err = s.nCfgClient.NetworkConfigs(s.ns).PatchMetricsExporter(netCfg)
	assert.NoError(c, err, fmt.Sprintf("failed to patch %v", netCfg.Name))

	// Create client pod and ensure deferred cleanup
	clientPod := s.metricsQueryClientPod(netCfg.Namespace, c)
	defer s.cleanupQueryClientPod(netCfg.Namespace, c)

	s.verifyMetricsPresentViaSvcSimple(netCfg, v1alpha1.ServiceTypeClusterIP, fieldsAndLabels, clientPod, c)
	s.verifyMetricsNotPresentViaSvcSimple(netCfg, v1alpha1.ServiceTypeClusterIP, cmFields, clientPod, c)
}

// TestExporterNICClients tests verification of NIC metrics from various NIC clients
func (s *E2ESuite) TestExporterNICClients(c *C) {
	_, err := s.nCfgClient.NetworkConfigs(s.ns).Get(s.cfgName, metav1.GetOptions{})
	assert.Errorf(c, err, fmt.Sprintf("config %v exists", s.cfgName))

	netCfg := s.getNetworkConfig()
	s.checkAndUseMockExporterImageForSim(netCfg)
	logger.Infof("create network-config %+v", netCfg.Spec.MetricsExporter)
	s.createNetworkConfig(netCfg, c)
	s.verifyMetricsExporterStatus(netCfg, c)

	// Create client pod and ensure deferred cleanup
	clientPod := s.metricsQueryClientPod(netCfg.Namespace, c)
	defer s.cleanupQueryClientPod(netCfg.Namespace, c)

	port_stats_labels := []string{"hostname", "nic_id", "pcie_bus_id", "port_id", "port_name", "serial_number"}
	lif_stats_labels := []string{"container", "eth_intf_name", "hostname", "namespace", "nic_id", "pcie_bus_id", "pod", "port_name", "serial_number"}
	rdma_stats_labels := []string{"container", "eth_intf_alias", "eth_intf_name", "hostname", "namespace", "nic_id", "pcie_bus_id", "pod", "rdma_dev_name", "serial_number"}
	ethtool_stats_labels := []string{"container", "eth_intf_alias", "eth_intf_name", "hostname", "namespace", "nic_id", "pcie_bus_id", "pod", "rdma_dev_name", "serial_number"}
	// Verify default NIC metrics using field-label pairs (excluding eth_ metrics and rdma_ metrics)
	// These should be there even without host networking enabled
	defaultFieldsAndLabels := map[string][]string{
		"nic_port_stats_frames_rx_all":            port_stats_labels,
		"nic_port_stats_frames_tx_all":            port_stats_labels,
		"nic_port_stats_frames_rx_bad_all":        port_stats_labels,
		"nic_lif_stats_rx_broadcast_drop_packets": lif_stats_labels,
		"nic_lif_stats_rx_unicast_packets":        lif_stats_labels,
	}
	s.verifyMetricsPresentViaSvcSimple(netCfg, v1alpha1.ServiceTypeClusterIP, defaultFieldsAndLabels, clientPod, c)

	// Enable host network for ethtool metrics
	hostNetwork := true
	netCfg.Spec.MetricsExporter.HostNetwork = &hostNetwork
	netCfg, err = s.nCfgClient.NetworkConfigs(s.ns).PatchMetricsExporter(netCfg)
	assert.NoError(c, err, fmt.Sprintf("failed to patch %v", netCfg.Name))
	s.verifyMetricsExporterStatus(netCfg, c)

	// Verify ethtool and rdma stats metrics with HostNetwork enabled
	ethtoolFieldsAndLabels := map[string][]string{
		"eth_rx_bytes":          ethtool_stats_labels,
		"eth_rx_packets":        ethtool_stats_labels,
		"eth_tx_bytes":          ethtool_stats_labels,
		"eth_tx_packets":        ethtool_stats_labels,
		"rdma_req_rx_cqe_err":   rdma_stats_labels,
		"rdma_req_rx_cqe_flush": rdma_stats_labels,
		"rdma_req_tx_loc_err":   rdma_stats_labels,
		"rdma_tx_ucast_pkts":    rdma_stats_labels,
	}
	s.verifyMetricsPresentViaSvcSimple(netCfg, v1alpha1.ServiceTypeClusterIP, ethtoolFieldsAndLabels, clientPod, c)
}

// TestServiceMonitorCreation verifies ServiceMonitor CR creation and fields
func (s *E2ESuite) TestServiceMonitorCreation(c *C) {
	// Ensure CRD installed
	err := utils.DeployResourcesFromFile(serviceMonitorCRDURL, s.k8sClientSet, s.apiClientSet, true)
	assert.NoError(c, err)
	defer func() {
		if errDel := utils.DeployResourcesFromFile(serviceMonitorCRDURL, s.k8sClientSet, s.apiClientSet, false); errDel != nil {
			logger.Errorf("failed to delete resources from %s: %+v", serviceMonitorCRDURL, errDel)
		}
	}()

	// Build NetworkConfig with ServiceMonitor enabled
	netCfg := s.getNetworkConfig()
	netCfg.Name = "nc-kuberbac-nodeport"
	netCfg.Spec.MetricsExporter.RbacConfig = v1alpha1.KubeRbacConfig{
		Enable:       boolPtr(true),
		DisableHttps: boolPtr(false),
	}
	netCfg.Spec.MetricsExporter.Prometheus = &v1alpha1.PrometheusConfig{
		ServiceMonitor: &v1alpha1.ServiceMonitorConfig{
			Enable:   ptr.To(true),
			Interval: "30s",
			Labels:   map[string]string{"custom": "label"},
		},
	}

	logger.Info("create nc-kuberbac-nodeport")
	s.createNetworkConfig(netCfg, c)
	s.verifyMetricsExporterStatus(netCfg, c)

	smName := netCfg.Name + "-" + metricsexporter.ExporterName
	var sm *monitoringv1.ServiceMonitor
	assert.Eventually(c, func() bool {
		var getErr error
		sm, getErr = s.monClient.MonitoringV1().ServiceMonitors(s.ns).Get(context.TODO(), smName, metav1.GetOptions{})
		return getErr == nil
	}, 1*time.Minute, 5*time.Second)

	// Validate metadata labels
	c.Assert(sm.Labels["custom"], Equals, "label")
	c.Assert(sm.Labels["app"], Equals, "amd-device-metrics-exporter")

	// Validate selector matches underlying Service
	svc, err := s.k8sClientSet.CoreV1().Services(s.ns).Get(context.TODO(), smName, metav1.GetOptions{})
	assert.NoError(c, err)
	for k, v := range sm.Spec.Selector.MatchLabels {
		c.Assert(svc.Labels[k], Equals, v)
	}

	// Validate scrape endpoint interval
	c.Assert(sm.Spec.Endpoints[0].Interval, Equals, monitoringv1.Duration("30s"))
}

// TestServiceMonitorCRDFlow tests failure if CRD missing and success after install
func (s *E2ESuite) TestServiceMonitorCRDFlow(c *C) {
	// Remove CRD if present
	err := utils.DeployResourcesFromFile(serviceMonitorCRDURL, s.k8sClientSet, s.apiClientSet, false)
	assert.NoError(c, err)

	netCfg := s.getNetworkConfig()
	netCfg.Name = "nc-kuberbac-nodeport"
	netCfg.Spec.MetricsExporter.RbacConfig = v1alpha1.KubeRbacConfig{
		Enable:       boolPtr(true),
		DisableHttps: boolPtr(false),
	}
	netCfg.Spec.MetricsExporter.Prometheus = &v1alpha1.PrometheusConfig{
		ServiceMonitor: &v1alpha1.ServiceMonitorConfig{Enable: ptr.To(true)},
	}

	logger.Info("create nc-kuberbac-nodeport")
	s.createNetworkConfig(netCfg, c)

	assert.Eventually(c, func() bool {
		d2, getErr := s.nCfgClient.NetworkConfigs(s.ns).Get("nc-kuberbac-nodeport", metav1.GetOptions{})
		if getErr != nil {
			return false
		}
		for _, cond := range d2.Status.Conditions {
			if cond.Type == conditions.ConditionTypeError &&
				cond.Status == metav1.ConditionTrue &&
				cond.Reason == conditions.ValidationError {
				return true
			}
		}
		return false
	}, 1*time.Minute, 5*time.Second)

	// Install CRD
	err = utils.DeployResourcesFromFile(serviceMonitorCRDURL, s.k8sClientSet, s.apiClientSet, true)
	assert.NoError(c, err)
	defer func() {
		errDel := utils.DeployResourcesFromFile(serviceMonitorCRDURL, s.k8sClientSet, s.apiClientSet, false)
		if errDel != nil {
			logger.Errorf("failed to delete resources from %s: %+v", serviceMonitorCRDURL, errDel)
		}
	}()

	// Re-create NetworkConfig
	s.deleteNetworkConfig(netCfg, c)
	s.createNetworkConfig(netCfg, c)

	// Now ServiceMonitor should be created
	smName := netCfg.Name + "-" + metricsexporter.ExporterName
	assert.Eventually(c, func() bool {
		_, getErr := s.monClient.MonitoringV1().ServiceMonitors(s.ns).Get(context.TODO(), smName, metav1.GetOptions{})
		return getErr == nil
	}, 1*time.Minute, 5*time.Second)
}

// TestKubeRbacProxyClusterIP tests the functionality of metrics exporter with kube-rbac-proxy and ClusterIP service
func (s *E2ESuite) TestKubeRbacProxyClusterIP(c *C) {
	_, err := s.nCfgClient.NetworkConfigs(s.ns).Get("nc-kuberbac-clusterip", metav1.GetOptions{})
	assert.Errorf(c, err, "config nc-kuberbac-clusterip exists")

	netCfg := s.getNetworkConfig()
	s.checkAndUseMockExporterImageForSim(netCfg)
	netCfg.Name = "nc-kuberbac-clusterip"
	netCfg.Spec.MetricsExporter.RbacConfig = v1alpha1.KubeRbacConfig{
		Enable:       boolPtr(true),
		DisableHttps: boolPtr(false),
	}
	logger.Info("create nc-kuberbac-clusterip")
	s.createNetworkConfig(netCfg, c)
	s.verifyMetricsExporterStatus(netCfg, c)

	err = utils.DeployResourcesFromFile("clusterrole_kuberbac.yaml", s.k8sClientSet, s.apiClientSet, true)
	assert.NoError(c, err, fmt.Sprintf("failed to deploy resources from clusterrole_kuberbac.yaml: %+v", err))

	// Create client pod to fetch metrics from
	clientPod := s.metricsQueryClientPod("metrics-reader", c)
	defer s.cleanupQueryClientPod(netCfg.Namespace, c)

	// Test metrics fetch
	s.verifyMetricsPresentViaSvc(
		netCfg,
		v1alpha1.ServiceTypeClusterIP,
		nil, // Use default fields and labels
		"$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)",
		true,  // HTTPS enabled
		false, // TLS verification disabled
		"",    // No CA cert path for this test
		"",    // No client cert path
		"",    // No client key path
		clientPod,
		c,
	)

	// delete
	s.deleteNetworkConfig(netCfg, c)
	err = utils.DeployResourcesFromFile("clusterrole_kuberbac.yaml", s.k8sClientSet, s.apiClientSet, false)
	assert.NoError(c, err, fmt.Sprintf("failed to delete resources from clusterrole_kuberbac.yaml: %+v", err))
}

// TestKubeRbacProxyNodePort tests the functionality of metrics exporter with kube-rbac-proxy and NodePort service
func (s *E2ESuite) TestKubeRbacProxyNodePort(c *C) {
	_, err := s.nCfgClient.NetworkConfigs(s.ns).Get("nc-kuberbac-nodeport", metav1.GetOptions{})
	assert.Errorf(c, err, "config networkconfig-kuberbac-nodeport exists")

	logger.Info("create networkconfig-kuberbac-nodeport")
	netCfg := s.getNetworkConfig()
	s.checkAndUseMockExporterImageForSim(netCfg)
	netCfg.Name = "nc-kuberbac-nodeport"
	netCfg.Spec.MetricsExporter.SvcType = "NodePort"
	netCfg.Spec.MetricsExporter.RbacConfig = v1alpha1.KubeRbacConfig{
		Enable:       boolPtr(true),
		DisableHttps: boolPtr(false),
	}
	s.createNetworkConfig(netCfg, c)
	s.verifyMetricsExporterStatus(netCfg, c)

	err = utils.DeployResourcesFromFile("clusterrole_kuberbac.yaml", s.k8sClientSet, s.apiClientSet, true)
	assert.NoError(c, err, fmt.Sprintf("failed to deploy resources from clusterrole_kuberbac.yaml: %+v", err))

	// Run the token request repeatedly
	token := ""
	assert.Eventually(c, func() bool {
		token, err = utils.GenerateServiceAccountToken(s.k8sClientSet, "default", "metrics-reader")
		if err != nil || len(token) == 0 {
			logger.Errorf("failed to generate token for default serviceaccount in metrics-client: %+v", err)
			return false
		}
		return true
	}, 1*time.Minute, 10*time.Second)
	assert.NoError(c, err, fmt.Sprintf("failed to generate token for default serviceaccount in metrics-client: %+v", err))

	// Verify metrics
	s.verifyMetricsPresentViaSvc(
		netCfg,
		v1alpha1.ServiceTypeNodePort,
		nil, // Use default fields and labels
		token,
		true,  // HTTPS enabled
		false, // TLS verification disabled
		"",    // No CA cert path for this test
		"",    // No client cert path
		"",    // No client key path
		nil,   // No client pod for NodePort
		c,
	)

	// Change the ports to give time for the old pods to be deleted and not affect the current test
	netCfg.Spec.MetricsExporter.RbacConfig.DisableHttps = boolPtr(true)
	netCfg.Spec.MetricsExporter.Port = 6000
	netCfg.Spec.MetricsExporter.NodePort = 32000
	netCfg, err = s.nCfgClient.NetworkConfigs(s.ns).PatchMetricsExporter(netCfg)
	assert.NoError(c, err, fmt.Sprintf("failed to patch %v", netCfg.Name))
	s.verifyMetricsExporterStatus(netCfg, c)

	// Verify metrics
	s.verifyMetricsPresentViaSvc(
		netCfg,
		v1alpha1.ServiceTypeNodePort,
		nil, // Use default fields and labels
		token,
		false, // HTTPS disabled
		false, // TLS verification disabled
		"",    // No CA cert path for this test
		"",    // No client cert path
		"",    // No client key path
		nil,   // No client pod for NodePort
		c,
	)

	// delete
	s.deleteNetworkConfig(netCfg, c)
	err = utils.DeployResourcesFromFile("clusterrole_kuberbac.yaml", s.k8sClientSet, s.apiClientSet, false)
	assert.NoError(c, err, fmt.Sprintf("failed to delete resources from clusterrole_kuberbac.yaml: %+v", err))
}

// TestKubeRbacProxyNodePortCerts tests the functionality of metrics exporter with kube-rbac-proxy, NodePort service and TLS certs (full TLS verification)
func (s *E2ESuite) TestKubeRbacProxyNodePortCerts(c *C) {
	_, err := s.nCfgClient.NetworkConfigs(s.ns).Get("nc-kuberbac-nodeport", metav1.GetOptions{})
	assert.Errorf(c, err, "config nc-kuberbac-nodeport exists")

	// Create the cacert, cert and private key
	caCert, serverCert, serverKey, _, _, err := s.setupKubeRbacCerts(c, false)
	assert.NoErrorf(c, err, "failed to generate certs")

	secretName := "kube-tls-secret"
	err = utils.CreateTLSSecret(context.TODO(), s.k8sClientSet, secretName, s.ns, serverCert, serverKey)
	assert.NoErrorf(c, err, fmt.Sprintf("failed to create secret %v", err))

	logger.Info("create nc-kuberbac-nodeport")
	netCfg := s.getNetworkConfig()
	s.checkAndUseMockExporterImageForSim(netCfg)
	netCfg.Name = "nc-kuberbac-nodeport"
	netCfg.Spec.MetricsExporter.SvcType = "NodePort"
	netCfg.Spec.MetricsExporter.RbacConfig = v1alpha1.KubeRbacConfig{
		Enable:       boolPtr(true),
		DisableHttps: boolPtr(false),
	}
	netCfg.Spec.MetricsExporter.RbacConfig.Secret = &v1.LocalObjectReference{Name: secretName}
	s.createNetworkConfig(netCfg, c)
	s.verifyMetricsExporterStatus(netCfg, c)

	err = utils.DeployResourcesFromFile("clusterrole_kuberbac.yaml", s.k8sClientSet, s.apiClientSet, true)
	assert.NoError(c, err, fmt.Sprintf("failed to deploy resources from clusterrole_kuberbac.yaml: %+v", err))

	// Run the token request repeatedly
	token := ""
	assert.Eventually(c, func() bool {
		token, err = utils.GenerateServiceAccountToken(s.k8sClientSet, "default", "metrics-reader")
		if err != nil || len(token) == 0 {
			logger.Errorf("failed to generate token for default serviceaccount in metrics-client: %+v", err)
			return false
		}
		return true
	}, 1*time.Minute, 10*time.Second)
	assert.NoError(c, err, fmt.Sprintf("failed to generate token for default serviceaccount in metrics-client: %+v", err))

	file, err := utils.CreateTempFile("cacert-*.crt", caCert)
	assert.NoError(c, err, fmt.Sprintf("failed to create cacert file: %v", err))

	// Run the curl job repeatedly using nodeport
	s.verifyMetricsPresentViaSvc(
		netCfg,
		v1alpha1.ServiceTypeNodePort,
		nil, // Use default fields and labels
		token,
		true,        // HTTPS enabled
		true,        // TLS verification enabled
		file.Name(), // CA cert path
		"",          // No client cert path
		"",          // No client key path
		nil,         // No client pod for NodePort
		c,
	)
	err = utils.DeleteTempFile(file)
	assert.NoError(c, err, fmt.Sprintf("failed to delete cacert file: %v", err))

	// delete
	err = utils.DeleteTLSSecret(context.TODO(), s.k8sClientSet, secretName, s.ns)
	assert.NoErrorf(c, err, fmt.Sprintf("failed to delete secret %v", err))
	s.deleteNetworkConfig(netCfg, c)
	err = utils.DeployResourcesFromFile("clusterrole_kuberbac.yaml", s.k8sClientSet, s.apiClientSet, false)
	assert.NoError(c, err, fmt.Sprintf("failed to delete resources from clusterrole_kuberbac.yaml: %+v", err))
}

// TestKubeRbacProxyNodePortMTLS exercises mTLS auth with User binding
func (s *E2ESuite) TestKubeRbacProxyNodePortMTLS(c *C) {
	_, err := s.nCfgClient.NetworkConfigs(s.ns).Get("netcfg-kuberbac-nodeport", metav1.GetOptions{})
	assert.Errorf(c, err, "config netcfg-kuberbac-nodeport exists")

	// RBAC
	err = utils.DeployResourcesFromFile("clusterrole_mtls.yaml", s.k8sClientSet, s.apiClientSet, true)
	assert.NoError(c, err)
	defer func() {
		if errDel := utils.DeployResourcesFromFile("clusterrole_mtls.yaml", s.k8sClientSet, s.apiClientSet, false); errDel != nil {
			logger.Errorf("failed to delete resources from clusterrole_mtls.yaml: %+v", errDel)
		}
	}()

	// Certs
	caCert, serverCert, serverKey, clientCert, clientKey, err := s.setupKubeRbacCerts(c, true)
	assert.NoError(c, err)

	// Secret
	secretName := "kube-tls-secret"
	err = utils.CreateTLSSecret(context.TODO(), s.k8sClientSet, secretName, s.ns, serverCert, serverKey)
	assert.NoError(c, err)
	defer func() {
		if errDel := utils.DeleteTLSSecret(context.TODO(), s.k8sClientSet, secretName, s.ns); errDel != nil {
			logger.Errorf("failed to delete TLS secret %s: %+v", secretName, errDel)
		}
	}()

	// Client CA ConfigMap
	cmName := "client-ca-cm"
	err = utils.CreateConfigMap(context.TODO(), s.k8sClientSet, s.ns, cmName, map[string]string{"ca.crt": string(caCert)})
	assert.NoError(c, err)
	defer func() {
		if errDel := utils.DeleteConfigMap(context.TODO(), s.k8sClientSet, cmName, s.ns); errDel != nil {
			logger.Errorf("failed to delete ConfigMap %s: %+v", cmName, errDel)
		}
	}()

	// Temp files
	deleteFile := func(file *os.File) {
		if file != nil {
			if errDel := utils.DeleteTempFile(file); errDel != nil {
				logger.Errorf("failed to delete temp file %s: %+v", file.Name(), errDel)
			}
		}
	}
	caFile, err := utils.CreateTempFile("cacert-*.crt", caCert)
	assert.NoError(c, err)
	defer deleteFile(caFile)
	certFile, err := utils.CreateTempFile("client-*.crt", clientCert)
	assert.NoError(c, err)
	defer deleteFile(certFile)
	keyFile, err := utils.CreateTempFile("client-*.key", clientKey)
	assert.NoError(c, err)
	defer deleteFile(keyFile)

	// NetworkConfig
	logger.Info("create netcfg-kuberbac-nodeport")
	netCfg := s.getNetworkConfig()
	s.checkAndUseMockExporterImageForSim(netCfg)
	netCfg.Name = "netcfg-kuberbac-nodeport"
	netCfg.Spec.MetricsExporter.SvcType = "NodePort"
	netCfg.Spec.MetricsExporter.RbacConfig = v1alpha1.KubeRbacConfig{
		Enable:            boolPtr(true),
		DisableHttps:      boolPtr(false),
		Secret:            &v1.LocalObjectReference{Name: secretName},
		ClientCAConfigMap: &v1.LocalObjectReference{Name: cmName},
	}
	s.createNetworkConfig(netCfg, c)
	s.verifyMetricsExporterStatus(netCfg, c)

	// Curl mTLS
	s.verifyMetricsPresentViaSvc(
		netCfg,
		v1alpha1.ServiceTypeNodePort,
		nil,             // Use default fields and labels
		"",              // No token for mTLS
		true,            // HTTPS enabled
		false,           // TLS verification enabled
		caFile.Name(),   // CA cert path
		certFile.Name(), // Client cert path
		keyFile.Name(),  // Client key path
		nil,             // No client pod for NodePort
		c,
	)
}

// TestKubeRbacProxyNodePortMTLSWithStaticAuth verifies static-auth mapping
func (s *E2ESuite) TestKubeRbacProxyNodePortMTLSWithStaticAuth(c *C) {
	_, err := s.nCfgClient.NetworkConfigs(s.ns).Get("netcfg-kuberbac-nodeport", metav1.GetOptions{})
	assert.Errorf(c, err, "config netcfg-kuberbac-nodeport exists")

	// No RBAC setup necessary
	// Certs
	caCert, serverCert, serverKey, clientCert, clientKey, err := s.setupKubeRbacCerts(c, true)
	assert.NoError(c, err)

	// Secret
	secretName := "kube-tls-secret"
	err = utils.CreateTLSSecret(context.TODO(), s.k8sClientSet, secretName, s.ns, serverCert, serverKey)
	assert.NoError(c, err)
	defer func() {
		if errDel := utils.DeleteTLSSecret(context.TODO(), s.k8sClientSet, secretName, s.ns); errDel != nil {
			logger.Errorf("failed to delete TLS secret %s: %+v", secretName, errDel)
		}
	}()

	cmName := "client-ca-cm"
	err = utils.CreateConfigMap(context.TODO(), s.k8sClientSet, s.ns, cmName, map[string]string{"ca.crt": string(caCert)})
	assert.NoError(c, err)
	defer func() {
		if errDel := utils.DeleteConfigMap(context.TODO(), s.k8sClientSet, cmName, s.ns); errDel != nil {
			logger.Errorf("failed to delete ConfigMap %s: %+v", cmName, errDel)
		}
	}()

	// Files
	deleteFile := func(file *os.File) {
		if file != nil {
			if errDel := utils.DeleteTempFile(file); errDel != nil {
				logger.Errorf("failed to delete temp file %s: %+v", file.Name(), errDel)
			}
		}
	}
	caFile, err := utils.CreateTempFile("cacert-*.crt", caCert)
	assert.NoError(c, err)
	defer deleteFile(caFile)
	certFile, err := utils.CreateTempFile("client-*.crt", clientCert)
	assert.NoError(c, err)
	defer deleteFile(certFile)
	keyFile, err := utils.CreateTempFile("client-*.key", clientKey)
	assert.NoError(c, err)
	defer deleteFile(keyFile)

	// NetworkConfig w/static-auth
	logger.Info("create netcfg-kuberbac-nodeport")
	netCfg := s.getNetworkConfig()
	s.checkAndUseMockExporterImageForSim(netCfg)
	netCfg.Name = "netcfg-kuberbac-nodeport"
	netCfg.Spec.MetricsExporter.SvcType = "NodePort"
	netCfg.Spec.MetricsExporter.RbacConfig = v1alpha1.KubeRbacConfig{
		Enable:              boolPtr(true),
		DisableHttps:        boolPtr(false),
		Secret:              &v1.LocalObjectReference{Name: secretName},
		ClientCAConfigMap:   &v1.LocalObjectReference{Name: cmName},
		StaticAuthorization: &v1alpha1.StaticAuthConfig{Enable: true, ClientName: "metrics-reader"},
	}
	s.createNetworkConfig(netCfg, c)
	s.verifyMetricsExporterStatus(netCfg, c)

	// Curl mTLS+static-auth
	s.verifyMetricsPresentViaSvc(
		netCfg,
		v1alpha1.ServiceTypeNodePort,
		nil,             // Use default fields and labels
		"",              // No token for mTLS
		true,            // HTTPS enabled
		false,           // TLS verification enabled
		caFile.Name(),   // CA cert path
		certFile.Name(), // Client cert path
		keyFile.Name(),  // Client key path
		nil,             // No client pod for NodePort
		c,
	)
}
