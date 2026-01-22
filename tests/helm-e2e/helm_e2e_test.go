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

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/stretchr/testify/assert"
	. "gopkg.in/check.v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ROCm/network-operator/api/v1alpha1"
	utils "github.com/ROCm/network-operator/internal"
)

const (
	defaultNetworkConfigName = "test-networkconfig"
	defaultNetworkConfig     = `../config/yamls/networkconfig.yaml`
	defaultDeviceConfig      = `../config/yamls/deviceconfig.yaml`
	defaultDeviceConfigName  = "test-deviceconfig"
	gpuOperatorNamespace     = "kube-amd-gpu"
	gpuOperatorReleaseName   = "amd-gpu-operator"
)

func (s *E2ESuite) installHelmChart(c *C, releaseName, namespace, chartPath string, expectErr bool, extraArgs []string) {
	args := []string{"install", releaseName, "-n", namespace, chartPath}
	args = append(args, extraArgs...)
	args = append(args, "--create-namespace")
	if s.simEnable {
		args = append(args, "--set", "controllerManager.env.simEnable=true")
	}

	cmd := exec.Command("helm", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	logger.Infof("Running command %+v", cmd.String())
	if err := cmd.Run(); err != nil && !expectErr {
		c.Fatalf("failed to install helm chart err %+v %+v", err, stderr.String())
	}
}

func (s *E2ESuite) uninstallHelmChart(c *C, releaseName, namespace string, expectErr bool, extraArgs []string) {
	args := []string{"delete", releaseName, "-n", namespace}
	args = append(args, extraArgs...)
	cmd := exec.Command("helm", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	logger.Infof("Running command %+v", cmd.String())
	if err := cmd.Run(); err != nil && !expectErr {
		c.Fatalf("failed to uninstall helm chart err %+v %+v", err, stderr.String())
	}
}

func (s *E2ESuite) upgradeHelmChart(c *C, releaseName, namespace, chartPath string, expectErr bool, extraArgs []string) {
	args := []string{"upgrade", releaseName, "-n", namespace, chartPath}
	args = append(args, extraArgs...)
	cmd := exec.Command("helm", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	logger.Infof("Running command %+v", cmd.String())
	if err := cmd.Run(); err != nil && !expectErr {
		c.Fatalf("failed to upgrade helm chart err %+v %+v", err, stderr.String())
	}
}

// pullGpuOperatorChart downloads the GPU operator helm chart asset from specified hourly build to a temp directory
func (s *E2ESuite) pullGpuOperatorChart(c *C) string {
	// Split the combined E2E_GPU_OP_HELM_CHART variable (format: tag/filename)
	parts := strings.Split(gpuOpHourlyBuildHelmChart, "/")
	if len(parts) != 2 {
		c.Fatalf("E2E_GPU_OP_HELM_CHART should be in format 'tag/filename', got: %s", gpuOpHourlyBuildHelmChart)
	}
	gpuOpAssetHourlyTag := parts[0]
	gpuOpHelmChartFileName := parts[1]

	logger.Infof("Pulling GPU operator helm chart: %s with tag: %s", gpuOpHelmChartFileName, gpuOpAssetHourlyTag)

	tmpDir := c.MkDir()

	// Set the target path in tmp directory
	targetPath := filepath.Join(tmpDir, "gpu-operator-helm-k8s-v0.0.1.tgz")

	// Check if asset-pull is available in PATH
	assetPullPath := "asset-pull"
	if _, err := exec.LookPath("asset-pull"); err != nil {
		logger.Infof("asset-pull not found in PATH, downloading from remote...")
		// Download asset-pull to temp directory
		assetPullPath = filepath.Join(tmpDir, "asset-pull")
		curlCmd := exec.Command("curl", "-o", assetPullPath, "http://pm.test.pensando.io/tools/asset-pull")
		if output, err := curlCmd.CombinedOutput(); err != nil {
			c.Fatalf("Failed to download asset-pull: %v\nOutput: %s", err, string(output))
		}
		// Make it executable
		if err := os.Chmod(assetPullPath, 0755); err != nil {
			c.Fatalf("Failed to make asset-pull executable: %v", err)
		}
		logger.Infof("Downloaded asset-pull to: %s", assetPullPath)
	}

	// Construct the asset-pull command
	cmd := exec.Command(assetPullPath,
		"-a", "assets-hq.pensando.io:9000",
		"-b", "builds",
		"-n", "/"+gpuOpHelmChartFileName,
		"hourly-gpu-operator",
		gpuOpAssetHourlyTag,
		targetPath)

	// Run the command
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.Fatalf("Failed to pull GPU operator helm chart: %v\nOutput: %s", err, string(output))
	}

	logger.Infof("Successfully pulled GPU operator helm chart to: %s", targetPath)
	return targetPath
}

func (s *E2ESuite) verifyNetworkConfig(c *C, testName string, expect bool,
	expectSpec *v1alpha1.NetworkConfigSpec,
	verifyFunc func(expect, actual *v1alpha1.NetworkConfigSpec) bool) {
	netCfgList, err := s.dClient.NetworkConfigs(s.ns).List(v1.ListOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		assert.NoError(c, err, fmt.Sprintf("test %v error listing NetworkConfig", testName))
	}
	if !expect && err != nil {
		// default CR was removed and even CRD was removed
		return
	}
	if !expect && err == nil && netCfgList != nil && len(netCfgList.Items) == 0 {
		// default CR was removed but CRD was not removed yet
		return
	}
	if expect && err == nil && netCfgList != nil {
		// make sure only one default CR exists
		assert.True(c, len(netCfgList.Items) == 1,
			"test %v expect only one default NetworkConfig but got %+v %+v",
			testName, len(netCfgList.Items), netCfgList.Items)
		// verify metadata
		assert.True(c, netCfgList.Items[0].Name == defaultNetworkConfigName,
			"test %v expect default NetworkConfig name to be %v but got %v",
			testName, defaultNetworkConfigName, netCfgList.Items[0].Name)
		assert.True(c, netCfgList.Items[0].Namespace == s.ns,
			"test %v expect default NetworkConfig namespace to be %v but got %v",
			testName, s.ns, netCfgList.Items[0].Namespace)
		// verify spec
		if expectSpec != nil && verifyFunc != nil {
			assert.True(c, verifyFunc(expectSpec, &netCfgList.Items[0].Spec),
				fmt.Sprintf("test %v expect %+v got %+v", testName, expectSpec, &netCfgList.Items[0].Spec))
		}
		return
	}
	c.Fatalf("test %v unexpected default CR, expect %+v list error %+v netCfgList %+v",
		testName, expect, err, netCfgList)
}

func (s *E2ESuite) verifyDeviceConfig(c *C, testName string, expectDeviceConfig bool) {
	var stdout, stderr bytes.Buffer
	args := []string{"get", "deviceconfig", "-n", gpuOperatorNamespace, "-o", "jsonpath={range .items[*]}{.metadata.name}{\" \"}{.metadata.namespace}{\"\\n\"}{end}"}
	cmd := exec.Command("kubectl", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	logger.Infof("Running command %+v", cmd.String())
	err := cmd.Run()
	if err != nil {
		// Check if this is an expected error (CRD removed)
		isExpectedError := strings.Contains(stderr.String(), "NotFound") ||
			strings.Contains(stderr.String(), "no matches") ||
			strings.Contains(stderr.String(), "doesn't have a resource type")

		if !isExpectedError {
			// Unexpected error - fail immediately
			c.Fatalf("test %v error listing DeviceConfig: %v %v", testName, err, stderr.String())
		}

		// Error when DeviceConfig is not expected
		if !expectDeviceConfig {
			// DeviceConfig and CRD is removed
			logger.Infof("test %v: DeviceConfig CR and CRD are both removed", testName)
			return
		}
		// We expect DeviceConfig but got error - fail
		c.Fatalf("test %v expect DeviceConfig to exist but got error: %v %v",
			testName, err, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	deviceConfigList := []string{}
	if output != "" {
		deviceConfigList = strings.Split(output, "\n")
	}

	if !expectDeviceConfig {
		// We don't expect DeviceConfig
		if len(deviceConfigList) == 0 {
			// CR was removed but CRD still exists (no error from kubectl)
			logger.Infof("test %v: DeviceConfig CR is removed but CRD still exists", testName)
			return
		}
		// Found DeviceConfigs when we shouldn't have
		c.Fatalf("test %v expect no DeviceConfig but got %d: %v",
			testName, len(deviceConfigList), deviceConfigList)
	}

	// We expect DeviceConfig and no error occurred
	// make sure only one default CR exists
	assert.True(c, len(deviceConfigList) == 1,
		"test %v expect only one default DeviceConfig but got %d: %v",
		testName, len(deviceConfigList), deviceConfigList)

	// Parse name and namespace from output (format: "name namespace")
	fields := strings.Fields(deviceConfigList[0])
	assert.True(c, len(fields) == 2,
		"test %v expect name and namespace in output but got: %v",
		testName, deviceConfigList[0])

	actualName := fields[0]
	actualNs := fields[1]

	// verify metadata - check the name
	assert.True(c, actualName == defaultDeviceConfigName,
		"test %v expect default DeviceConfig name to be %v but got %v",
		testName, defaultDeviceConfigName, actualName)

	// verify namespace
	assert.True(c, actualNs == gpuOperatorNamespace,
		"test %v expect default DeviceConfig namespace to be %v but got %v",
		testName, gpuOperatorNamespace, actualNs)
}

func (s *E2ESuite) createCR(c *C, configYaml string) {
	args := []string{"apply", "-f", configYaml}
	cmd := exec.Command("kubectl", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	logger.Infof("Running command %+v", cmd.String())
	if err := cmd.Run(); err != nil {
		c.Fatalf("failed to create CR err: %+v, %+v", err, stderr.String())
	}
}

func (s *E2ESuite) verifyHelmReleaseInstalled(c *C, releaseName, namespace string) {
	const (
		timeout = 5 * time.Minute
		tick    = 10 * time.Second
	)

	releaseInstalled := assert.Eventually(c, func() bool {
		args := []string{"status", releaseName, "-n", namespace}
		cmd := exec.Command("helm", args...)
		output, err := cmd.CombinedOutput()

		if err != nil {
			logger.Infof("helm release %s in namespace %s not found yet: %v", releaseName, namespace, string(output))
			return false
		}

		// Verify release status is "deployed"
		outputStr := string(output)
		if !strings.Contains(outputStr, "STATUS: deployed") {
			logger.Infof("helm release %s in namespace %s is not yet deployed", releaseName, namespace)
			return false
		}

		logger.Infof("helm release %s in namespace %s is successfully deployed", releaseName, namespace)
		return true
	}, timeout, tick, "expect helm release %s in namespace %s to be installed and deployed", releaseName, namespace)

	if !releaseInstalled {
		c.Fatalf("expect helm release %s in namespace %s to be installed and deployed but it is not", releaseName, namespace)
	}
}

func (s *E2ESuite) verifyHelmReleaseUninstalled(c *C, releaseName, namespace string) {
	const (
		timeout = 5 * time.Minute
		tick    = 10 * time.Second
	)

	releaseUninstalled := assert.Eventually(c, func() bool {
		args := []string{"status", releaseName, "-n", namespace}
		cmd := exec.Command("helm", args...)
		output, err := cmd.CombinedOutput()

		if err != nil {
			// Check if this is an expected error (release not found)
			isExpectedError := strings.Contains(string(output), "not found") ||
				strings.Contains(string(output), "Error:")

			if isExpectedError {
				logger.Infof("helm release %s in namespace %s is removed", releaseName, namespace)
				return true
			}
			// Unexpected error - log and retry
			logger.Infof("error checking helm release %s in namespace %s: %v %v", releaseName, namespace, err, string(output))
			return false
		}

		// Release still exists
		logger.Infof("helm release %s in namespace %s still exists, waiting for removal", releaseName, namespace)
		return false
	}, timeout, tick, "expect helm release %s in namespace %s to be uninstalled", releaseName, namespace)

	if !releaseUninstalled {
		c.Fatalf("expect helm release %s in namespace %s to be uninstalled but it still exists", releaseName, namespace)
	}
}

func (s *E2ESuite) verifyPods(c *C, namespace, labelSelector string) {
	const (
		timeout = 100 * time.Second
		tick    = 5 * time.Second
	)

	args := []string{"get", "pods", "-n", namespace, "-l", labelSelector, "-o", "jsonpath={range .items[*]}{.metadata.name} {.status.phase},{end}"}

	// check that pods exist and all are in Running status
	allPodsRunning := assert.Eventually(c, func() bool {
		cmd := exec.Command("kubectl", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			logger.Infof("failed to get pods with label %s in namespace %s: %v %v", labelSelector, namespace, err, string(output))
			return false
		}

		allPods := string(output)
		if allPods == "" {
			logger.Infof("no pods created yet with label %s in namespace %s", labelSelector, namespace)
			return false
		}

		// Check if any pods are not in Running status
		cmd = exec.Command("kubectl", append(args, "--field-selector=status.phase!=Running")...)
		output, err = cmd.CombinedOutput()
		if err != nil {
			logger.Infof("failed to get pods with label %s in namespace %s: %v %v", labelSelector, namespace, err, string(output))
			return false
		}

		nonRunningPods := string(output)
		if nonRunningPods != "" {
			logger.Infof("some pods with label %s in namespace %s not in Running status: %s", labelSelector, namespace, nonRunningPods)
			return false
		}

		logger.Infof("all pods with label %s in namespace %s are in Running status", labelSelector, namespace)
		return true
	}, timeout, tick, "expect all pods with label %s in namespace %s to exist and be in Running status", labelSelector, namespace)

	if !allPodsRunning {
		c.Fatalf("expect all pods with label %s in namespace %s to exist and be in Running status but some are not", labelSelector, namespace)
	}
}

// basic test: install network operator, create CR and verify the pods are runing, uninstall network operator and ensure CR is removed
func (s *E2ESuite) TestHelmInstallUnInstall(c *C) {
	testCases := []struct {
		name          string
		withoutKMM    bool
		withoutNFD    bool
		withoutMultus bool
	}{
		{
			name: "AllEnabled",
		},
		// {
		// 	name:       "WithoutKMM",
		// 	withoutKMM: true,
		// },
		{
			name:       "WithoutNFD",
			withoutNFD: true,
		},
		// {
		// 	name:          "WithoutMultus",
		// 	withoutMultus: true,
		// },
	}

	for _, tc := range testCases {
		logger.Infof("Running test case: %s", tc.name)

		var extraArgs []string
		if tc.withoutKMM {
			extraArgs = append(extraArgs, "--set", "kmm.enabled=false")
		}
		if tc.withoutNFD {
			extraArgs = append(extraArgs, "--set", "node-feature-discovery.enabled=false,installdefaultNFDRule=false")
		}
		if tc.withoutMultus {
			extraArgs = append(extraArgs, "--set", "multus.enabled=false")
		}

		testName := fmt.Sprintf("TestHelmInstallUnInstall/%s", tc.name)
		s.installHelmChart(c, s.releaseName, s.ns, networkOperatorChart, false, extraArgs)
		// verify helm release is installed
		s.verifyHelmReleaseInstalled(c, s.releaseName, s.ns)
		// create CR
		s.createCR(c, defaultNetworkConfig)
		s.verifyNetworkConfig(c, testName, true, nil, nil)
		// verify that the pods are running for the CR
		s.verifyPods(c, s.ns, fmt.Sprintf("%s=%s", utils.CRNameLabel, defaultNetworkConfigName))
		// uninstall network operator
		s.uninstallHelmChart(c, s.releaseName, s.ns, false, nil)
		// verify CR was removed
		s.verifyNetworkConfig(c, testName, false, nil, nil)
		// verify helm release was removed
		s.verifyHelmReleaseUninstalled(c, s.releaseName, s.ns)
	}
}

// tests helm upgrade
func (s *E2ESuite) TestHelmUpgrade(c *C) {
	s.installHelmChart(c, s.releaseName, s.ns, networkOperatorChart, false, []string{})
	s.verifyNetworkConfig(c, "TestHelmUpgrade", false, nil, nil)
	s.createCR(c, defaultNetworkConfig)
	s.verifyNetworkConfig(c, "TestHelmUpgrade", true, nil, nil)
	logger.Infof("wait 30s for the operands to be created")
	time.Sleep(30 * time.Second)
	s.verifyPods(c, s.ns, fmt.Sprintf("%s=%s", utils.CRNameLabel, defaultNetworkConfigName))
	s.upgradeHelmChart(c, s.releaseName, s.ns, networkOperatorChart, false, nil)
	// verify that existing CR is not affected by upgrade
	s.verifyNetworkConfig(c, "TestHelmUpgrade", true, nil, nil)
	s.verifyPods(c, s.ns, fmt.Sprintf("%s=%s", utils.CRNameLabel, defaultNetworkConfigName))
	s.uninstallHelmChart(c, s.releaseName, s.ns, false, nil)
	// verify CR was removed
	s.verifyNetworkConfig(c, "TestHelmUpgrade", false, nil, nil)
	// verify helm release was removed
	s.verifyHelmReleaseUninstalled(c, s.releaseName, s.ns)

	// install again to verify the flow works for 2nd time
	logger.Infof("installing helm chart again to verify the flow works for 2nd time")
	s.installHelmChart(c, s.releaseName, s.ns, networkOperatorChart, false, nil)
	s.verifyNetworkConfig(c, "TestHelmUpgrade", false, nil, nil)
	s.createCR(c, defaultNetworkConfig)
	s.verifyNetworkConfig(c, "TestHelmUpgrade", true, nil, nil)
	logger.Infof("wait 30s for the operands to be created")
	time.Sleep(30 * time.Second)
	s.verifyPods(c, s.ns, fmt.Sprintf("%s=%s", utils.CRNameLabel, defaultNetworkConfigName))
	s.upgradeHelmChart(c, s.releaseName, s.ns, networkOperatorChart, false, nil)
	s.verifyNetworkConfig(c, "TestHelmUpgrade", true, nil, nil)
	s.verifyPods(c, s.ns, fmt.Sprintf("%s=%s", utils.CRNameLabel, defaultNetworkConfigName))
	s.uninstallHelmChart(c, s.releaseName, s.ns, false, nil)
	s.verifyNetworkConfig(c, "TestHelmUpgrade", false, nil, nil)
	s.verifyHelmReleaseUninstalled(c, s.releaseName, s.ns)
}

// TestOperatorsCoexistence tests the coexistence of AMD Network Operator and AMD GPU Operator
func (s *E2ESuite) TestOperatorsCoexistence(c *C) {
	testCases := []struct {
		name                string
		reverseInstallOrder bool // Default: GPU operator first, then Network operator, if true: Opposite order
	}{
		{
			name: "InstallGPUOP-InstallNetOP",
		},
		{
			name:                "InstallNetOP-InstallGPUOP",
			reverseInstallOrder: true,
		},
	}

	// Pull GPU operator helm chart
	gpuOperatorChart := s.pullGpuOperatorChart(c)

	for _, tc := range testCases {
		func() {
			logger.Infof("Running test case: %s", tc.name)
			testName := fmt.Sprintf("TestOperatorsCoexistence/%s", tc.name)

			kmmNamespace := gpuOperatorNamespace
			nfdNamespace := gpuOperatorNamespace

			gpuOpExtraArgs := []string{"--set", "crds.defaultCR.install=false"}
			netOpExtraArgs := []string{}

			// The first operator installed provides KMM and NFD, the second uses them
			if !tc.reverseInstallOrder {
				// GPU operator first: enable KMM and NFD in GPU operator
				gpuOpExtraArgs = append(gpuOpExtraArgs, "--set", "kmm.enabled=true")
				gpuOpExtraArgs = append(gpuOpExtraArgs, "--set", "node-feature-discovery.enabled=true,installdefaultNFDRule=true")
				netOpExtraArgs = append(netOpExtraArgs, "--set", "kmm.enabled=false")
				netOpExtraArgs = append(netOpExtraArgs, "--set", "node-feature-discovery.enabled=false,installdefaultNFDRule=false")
			} else {
				// Network operator first: enable KMM and NFD in Network operator
				netOpExtraArgs = append(netOpExtraArgs, "--set", "kmm.enabled=true")
				netOpExtraArgs = append(netOpExtraArgs, "--set", "node-feature-discovery.enabled=true,installdefaultNFDRule=true")
				gpuOpExtraArgs = append(gpuOpExtraArgs, "--set", "kmm.enabled=false")
				gpuOpExtraArgs = append(gpuOpExtraArgs, "--set", "node-feature-discovery.enabled=false,installdefaultNFDRule=false")
				kmmNamespace = s.ns
				nfdNamespace = s.ns
			}

			if !tc.reverseInstallOrder {
				s.installHelmChart(c, gpuOperatorReleaseName, gpuOperatorNamespace, gpuOperatorChart, false, gpuOpExtraArgs)
				defer func() {
					s.uninstallHelmChart(c, gpuOperatorReleaseName, gpuOperatorNamespace, false, nil)
					s.verifyHelmReleaseUninstalled(c, gpuOperatorReleaseName, gpuOperatorNamespace)
				}()
				s.installHelmChart(c, s.releaseName, s.ns, networkOperatorChart, false, netOpExtraArgs)
				defer func() {
					s.uninstallHelmChart(c, s.releaseName, s.ns, false, nil)
					s.verifyHelmReleaseUninstalled(c, s.releaseName, s.ns)
				}()
			} else {
				s.installHelmChart(c, s.releaseName, s.ns, networkOperatorChart, false, netOpExtraArgs)
				defer func() {
					s.uninstallHelmChart(c, s.releaseName, s.ns, false, nil)
					s.verifyHelmReleaseUninstalled(c, s.releaseName, s.ns)
				}()
				s.installHelmChart(c, gpuOperatorReleaseName, gpuOperatorNamespace, gpuOperatorChart, false, gpuOpExtraArgs)
				defer func() {
					s.uninstallHelmChart(c, gpuOperatorReleaseName, gpuOperatorNamespace, false, nil)
					s.verifyHelmReleaseUninstalled(c, gpuOperatorReleaseName, gpuOperatorNamespace)
				}()
			}

			// Verify helm releases are installed
			s.verifyHelmReleaseInstalled(c, gpuOperatorReleaseName, gpuOperatorNamespace)
			s.verifyHelmReleaseInstalled(c, s.releaseName, s.ns)

			// Create network config CR
			s.createCR(c, defaultNetworkConfig)
			s.verifyNetworkConfig(c, testName, true, nil, nil)

			// Create device config CR
			s.createCR(c, defaultDeviceConfig)
			s.verifyDeviceConfig(c, testName, true)

			// Verify network operator controller pods are running
			s.verifyPods(c, s.ns, "app.kubernetes.io/name=network-operator-charts")

			// Verify gpu operator controller pods are running
			s.verifyPods(c, gpuOperatorNamespace, "app.kubernetes.io/name=gpu-operator-charts")

			// Verify KMM pods are running
			s.verifyPods(c, kmmNamespace, "app.kubernetes.io/name=kmm")

			// Verify NFD pods are running
			s.verifyPods(c, nfdNamespace, "app.kubernetes.io/name=node-feature-discovery")

			// Verify network operator's operands are running
			s.verifyPods(c, s.ns, fmt.Sprintf("%s=%s", utils.CRNameLabel, defaultNetworkConfigName))

			// Verify gpu operator's operands are running
			s.verifyPods(c, gpuOperatorNamespace, fmt.Sprintf("daemonset-name=%s", defaultDeviceConfigName))
		}()
	}
}
