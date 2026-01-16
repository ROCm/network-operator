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
	"fmt"

	v1alpha1 "github.com/ROCm/network-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	. "gopkg.in/check.v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// TestDriverInstallDefault tests the default driver installation
func (s *E2ESuite) TestDriverInstallDefault(c *C) {
	_, err := s.nCfgClient.NetworkConfigs(s.ns).Get(s.cfgName, metav1.GetOptions{})
	assert.Errorf(c, err, fmt.Sprintf("config %v exists", s.cfgName))

	logger.Infof("create %v", s.cfgName)
	netCfg := s.getNetworkConfig()
	netCfg.Spec.Selector = vnicselector
	netCfg.Spec.Driver.Version = ""
	s.createNetworkConfig(netCfg, c)
	s.verifyOperandReadiness(c, netCfg)
	s.verifyNodeDriverVersionLabel(netCfg, c)

	utilsDS := s.createDriverCheckUtilsDS(netCfg.Namespace, netCfg.Spec.Selector, c)
	defer s.cleanupDriverCheckUtilsDS(netCfg.Namespace, c)
	s.verifyIonicModuleVersion(netCfg, utilsDS, c)
	s.verifyIonicModuleBlacklistPresent(netCfg, false, utilsDS, c)

	// enable blacklist and verify the existence of blacklist file
	logger.Infof("enable driver blacklist")
	boolTrue := true
	boolFalse := false
	netCfg.Spec.Driver.Blacklist = &boolTrue
	netCfg.Spec.DevicePlugin.EnableNodeLabeller = &boolTrue
	netCfg, err = s.nCfgClient.NetworkConfigs(s.ns).PatchDriversBlacklist(netCfg)
	assert.NoError(c, err, "failed to patch network config for driver blacklist")
	s.verifyIonicModuleBlacklistPresent(netCfg, true, utilsDS, c)

	// disable blacklist and verify the absence of blacklist file
	logger.Infof("disable driver blacklist")
	netCfg.Spec.Driver.Blacklist = &boolFalse
	netCfg.Spec.DevicePlugin.EnableNodeLabeller = &boolTrue
	netCfg, err = s.nCfgClient.NetworkConfigs(s.ns).PatchDriversBlacklist(netCfg)
	assert.NoError(c, err, "failed to patch network config for driver un-blacklist")
	s.verifyIonicModuleBlacklistPresent(netCfg, false, utilsDS, c)

	// delete
	s.deleteNetworkConfig(netCfg, c)
	s.verifyIonicModuleNotPresent(netCfg, utilsDS, c)
}

// TestDriverUpgradeByUpdatingCR tests the driver upgrade by updating the CR
func (s *E2ESuite) TestDriverUpgradeByUpdatingCR(c *C) {
	_, err := s.nCfgClient.NetworkConfigs(s.ns).Get(s.cfgName, metav1.GetOptions{})
	assert.Errorf(c, err, fmt.Sprintf("network config %v exists", s.cfgName))

	logger.Infof("create %v", s.cfgName)
	netCfg := s.getNetworkConfig()
	netCfg.Spec.Selector = vnicselector
	s.createNetworkConfig(netCfg, c)
	s.verifyOperandReadiness(c, netCfg)
	s.verifyNodeDriverVersionLabel(netCfg, c)

	utilsDS := s.createDriverCheckUtilsDS(netCfg.Namespace, netCfg.Spec.Selector, c)
	defer s.cleanupDriverCheckUtilsDS(netCfg.Namespace, c)
	s.verifyIonicModuleVersion(netCfg, utilsDS, c)

	// upgrade
	// update the CR's driver version config
	netCfg.Spec.Driver.Version = "1.117.1-a-63"
	netCfg, err = s.nCfgClient.NetworkConfigs(s.ns).PatchDriversVersion(netCfg)
	assert.NoError(c, err, "failed to patch network config")
	s.verifyOperandReadiness(c, netCfg)
	s.verifyNodeDriverVersionLabel(netCfg, c)
	s.verifyIonicModuleVersion(netCfg, utilsDS, c)

	// delete
	s.deleteNetworkConfig(netCfg, c)
	s.verifyIonicModuleNotPresent(netCfg, utilsDS, c)
}

// TestDriverUpgradeByPushingNewCR tests the driver upgrade by pushing new CR
func (s *E2ESuite) TestDriverUpgradeByPushingNewCR(c *C) {
	_, err := s.nCfgClient.NetworkConfigs(s.ns).Get(s.cfgName, metav1.GetOptions{})
	assert.Errorf(c, err, fmt.Sprintf("network config %v exists", s.cfgName))

	logger.Infof("create %v", s.cfgName)
	netCfg := s.getNetworkConfig()
	netCfg.Spec.Selector = vnicselector
	s.createNetworkConfig(netCfg, c)

	s.verifyOperandReadiness(c, netCfg)
	s.verifyNodeDriverVersionLabel(netCfg, c)

	utilsDS := s.createDriverCheckUtilsDS(netCfg.Namespace, netCfg.Spec.Selector, c)
	defer s.cleanupDriverCheckUtilsDS(netCfg.Namespace, c)
	s.verifyIonicModuleVersion(netCfg, utilsDS, c)

	// delete network config
	s.deleteNetworkConfig(netCfg, c)
	s.verifyIonicModuleNotPresent(netCfg, utilsDS, c)

	// upgrade by pushing new CR with new version
	netCfg.Spec.Driver.Version = "1.117.1-a-63"
	s.createNetworkConfig(netCfg, c)
	s.verifyOperandReadiness(c, netCfg)
	s.verifyNodeDriverVersionLabel(netCfg, c)
	s.verifyIonicModuleVersion(netCfg, utilsDS, c)

	s.deleteNetworkConfig(netCfg, c)
	s.verifyIonicModuleNotPresent(netCfg, utilsDS, c)
}

func (s *E2ESuite) TestParallelUpgrade(c *C) {
	testCases := []struct {
		name          string
		fromVersion   string
		toVersion     string
		upgradePolicy v1alpha1.DriverUpgradePolicySpec
	}{
		{
			name:        "default version to specific version",
			fromVersion: "",
			toVersion:   "1.117.1-a-63",
			upgradePolicy: v1alpha1.DriverUpgradePolicySpec{
				Enable:              boolPtr(true),
				RebootRequired:      boolPtr(false),
				MaxUnavailableNodes: intstr.FromString("100%"),
			},
		},
		{
			name:        "specific version to default version",
			fromVersion: "1.117.1-a-63",
			toVersion:   "",
			upgradePolicy: v1alpha1.DriverUpgradePolicySpec{
				Enable:              boolPtr(true),
				RebootRequired:      boolPtr(false),
				MaxUnavailableNodes: intstr.FromString("100%"),
			},
		},
		{
			name:        "default upgrade policy",
			fromVersion: "1.117.1-a-42",
			toVersion:   "1.117.1-a-63",
			upgradePolicy: v1alpha1.DriverUpgradePolicySpec{
				Enable:              boolPtr(true),
				RebootRequired:      boolPtr(false),
				MaxUnavailableNodes: intstr.FromString("100%"),
			},
		},
		{
			name:        "upgrade two nodes in parallel",
			fromVersion: "1.117.1-a-42",
			toVersion:   "1.117.1-a-63",
			upgradePolicy: v1alpha1.DriverUpgradePolicySpec{
				Enable:              boolPtr(true),
				RebootRequired:      boolPtr(false),
				MaxParallelUpgrades: 2,
				MaxUnavailableNodes: intstr.FromString("100%"),
			},
		},
		{
			name:        "upgrade with drain policy",
			fromVersion: "1.117.1-a-42",
			toVersion:   "1.117.1-a-63",
			upgradePolicy: v1alpha1.DriverUpgradePolicySpec{
				Enable:              boolPtr(true),
				RebootRequired:      boolPtr(false),
				MaxParallelUpgrades: 2,
				MaxUnavailableNodes: intstr.FromString("100%"),
				NodeDrainPolicy: &v1alpha1.DrainSpec{
					Force:          boolPtr(true),
					TimeoutSeconds: 300,
				},
			},
		},
		{
			name:        "upgrade with pod deletion policy",
			fromVersion: "1.117.1-a-42",
			toVersion:   "1.117.1-a-63",
			upgradePolicy: v1alpha1.DriverUpgradePolicySpec{
				Enable:              boolPtr(true),
				RebootRequired:      boolPtr(false),
				MaxParallelUpgrades: 2,
				MaxUnavailableNodes: intstr.FromString("100%"),
				PodDeletionPolicy: &v1alpha1.PodDeletionSpec{
					Force:          boolPtr(true),
					TimeoutSeconds: 300,
				},
			},
		},
	}

	utilsDS := s.createDriverCheckUtilsDS(
		s.ns,
		vnicselector,
		c,
	)
	defer s.cleanupDriverCheckUtilsDS(s.ns, c)

	for _, tc := range testCases {
		logger.Infof("Running test case: %s", tc.name)

		_, err := s.nCfgClient.NetworkConfigs(s.ns).Get(s.cfgName, metav1.GetOptions{})
		assert.Errorf(c, err, fmt.Sprintf("config %v exists", s.cfgName))

		logger.Infof("create %v with version %s", s.cfgName, tc.fromVersion)
		netCfg := s.getNetworkConfig()
		netCfg.Spec.Selector = vnicselector
		netCfg.Spec.Driver.Version = tc.fromVersion
		netCfg.Spec.Driver.UpgradePolicy = &tc.upgradePolicy
		s.createNetworkConfig(netCfg, c)

		s.verifyOperandReadiness(c, netCfg)
		s.verifyNodeDriverVersionLabel(netCfg, c)
		s.verifyNodeModuleStatus(netCfg, v1alpha1.UpgradeStateInstallComplete, c)

		s.verifyIonicModuleVersion(netCfg, utilsDS, c)

		// upgrade
		logger.Infof("upgrading to version %s", tc.toVersion)
		netCfg.Spec.Driver.Version = tc.toVersion
		netCfg, err = s.nCfgClient.NetworkConfigs(s.ns).PatchDriversVersion(netCfg)
		assert.NoError(c, err, "failed to patch network config")
		s.verifyOperandReadiness(c, netCfg)
		s.verifyNodeDriverVersionLabel(netCfg, c)
		s.verifyNodeModuleStatus(netCfg, v1alpha1.UpgradeStateComplete, c)
		s.verifyIonicModuleVersion(netCfg, utilsDS, c)

		s.deleteNetworkConfig(netCfg, c)
		s.verifyIonicModuleNotPresent(netCfg, utilsDS, c)

		logger.Infof("Completed test case: %s", tc.name)
	}
}

func (s *E2ESuite) TestUpgradePolicyDuringUpgrade(c *C) {
	testCases := []struct {
		name          string
		upgradePolicy v1alpha1.DriverUpgradePolicySpec
	}{
		{
			name: "change max parallel during upgrade",
			upgradePolicy: v1alpha1.DriverUpgradePolicySpec{
				Enable:              boolPtr(true),
				RebootRequired:      boolPtr(false),
				MaxUnavailableNodes: intstr.FromString("100%"),
				MaxParallelUpgrades: 2, // change from default 1 to 2
			},
		},
		{
			name: "change max unavailable during upgrade",
			upgradePolicy: v1alpha1.DriverUpgradePolicySpec{
				Enable:              boolPtr(true),
				RebootRequired:      boolPtr(false),
				MaxParallelUpgrades: 1,
				MaxUnavailableNodes: intstr.FromString("50%"), // set to 50% during upgrade
			},
		},
		{
			name: "change reboot required during upgrade",
			upgradePolicy: v1alpha1.DriverUpgradePolicySpec{
				Enable:              boolPtr(true),
				RebootRequired:      boolPtr(true), // enable during upgrade
				MaxParallelUpgrades: 1,
				MaxUnavailableNodes: intstr.FromString("100%"),
			},
		},
	}

	utilsDS := s.createDriverCheckUtilsDS(
		s.ns,
		vnicselector,
		c,
	)
	defer s.cleanupDriverCheckUtilsDS(s.ns, c)

	for _, tc := range testCases {
		logger.Infof("Running test case: %s", tc.name)

		_, err := s.nCfgClient.NetworkConfigs(s.ns).Get(s.cfgName, metav1.GetOptions{})
		assert.Errorf(c, err, fmt.Sprintf("config %v exists", s.cfgName))

		fromVersion := "1.117.1-a-42"
		toVersion := "1.117.1-a-63"
		initialUpgradePolicy := v1alpha1.DriverUpgradePolicySpec{
			Enable:              boolPtr(true),
			RebootRequired:      boolPtr(false),
			MaxParallelUpgrades: 1,
			MaxUnavailableNodes: intstr.FromString("100%"),
		}

		logger.Infof("create %v with version %s", s.cfgName, fromVersion)
		netCfg := s.getNetworkConfig()
		netCfg.Spec.Selector = vnicselector
		netCfg.Spec.Driver.Version = fromVersion
		netCfg.Spec.Driver.UpgradePolicy = &initialUpgradePolicy
		s.createNetworkConfig(netCfg, c)

		s.verifyOperandReadiness(c, netCfg)
		s.verifyNodeDriverVersionLabel(netCfg, c)
		s.verifyNodeModuleStatus(netCfg, v1alpha1.UpgradeStateInstallComplete, c)
		s.verifyIonicModuleVersion(netCfg, utilsDS, c)

		// upgrade
		logger.Infof("upgrading to version %s", toVersion)
		netCfg.Spec.Driver.Version = toVersion
		netCfg, err = s.nCfgClient.NetworkConfigs(s.ns).PatchDriversVersion(netCfg)
		assert.NoError(c, err, "failed to patch network config")

		// patch the upgrade policy during upgrade
		netCfg.Spec.Driver.UpgradePolicy = &tc.upgradePolicy
		netCfg, err = s.nCfgClient.NetworkConfigs(s.ns).PatchUpgradePolicy(netCfg)
		assert.NoError(c, err, "failed to patch driver upgrade policy")

		s.verifyOperandReadiness(c, netCfg)
		s.verifyNodeDriverVersionLabel(netCfg, c)
		s.verifyNodeModuleStatus(netCfg, v1alpha1.UpgradeStateComplete, c)
		s.verifyIonicModuleVersion(netCfg, utilsDS, c)

		s.deleteNetworkConfig(netCfg, c)
		s.verifyIonicModuleNotPresent(netCfg, utilsDS, c)

		logger.Infof("Completed test case: %s", tc.name)
	}
}
