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

package kmmmodule

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	amdv1alpha1 "github.com/ROCm/network-operator/api/v1alpha1"
	utils "github.com/ROCm/network-operator/internal"
	kmmv1beta1 "github.com/rh-ecosystem-edge/kernel-module-management/api/v1beta1"
	"golang.org/x/exp/maps"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	networkDriverModuleName       = "ionic_rdma"
	pdsCoreModuleName             = "pds_core"
	tawkIPCModuleName             = "tawk_ipc"
	kmmNodeVersionLabelTemplate   = "kmm.node.kubernetes.io/version-module.%s.%s"
	defaultOcDriversImageTemplate = "image-registry.openshift-image-registry.svc:5000/$MOD_NAMESPACE/amdionic_kmod"
	// start local registry image-registry:5000 in k8s
	defaultDriversImageTemplate = "image-registry:5000/$MOD_NAMESPACE/amdionic_kmod"
	defaultOcDriversVersion     = "1.117.1-a-42"
	defaultInstallerRepoURL     = "https://repo.radeon.com"
	defaultInitContainerImage   = "busybox:1.36"
)

var (
	//go:embed dockerfiles/DockerfileTemplate.ubuntu
	dockerfileTemplateUbuntu string
	//go:embed dockerfiles/DockerfileTemplate.coreos
	buildOcDockerfile string
	//go:embed devdockerfiles/devdockerfile.txt
	dockerfileDevTemplateUbuntu string
)

//go:generate mockgen -source=kmmmodule.go -package=kmmmodule -destination=mock_kmmmodule.go KMMModuleAPI
type KMMModuleAPI interface {
	SetNodeVersionLabelAsDesired(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig, nodes *v1.NodeList) error
	SetBuildConfigMapAsDesired(buildCM *v1.ConfigMap, nwConfig *amdv1alpha1.NetworkConfig) error
	SetKMMModuleAsDesired(ctx context.Context, mod *kmmv1beta1.Module, nwConfig *amdv1alpha1.NetworkConfig, nodes *v1.NodeList) error
}

type kmmModule struct {
	client      client.Client
	scheme      *runtime.Scheme
	isOpenShift bool
}

func NewKMMModule(client client.Client, scheme *runtime.Scheme, isOpenShift bool) KMMModuleAPI {
	return &kmmModule{
		client:      client,
		scheme:      scheme,
		isOpenShift: isOpenShift,
	}
}

func (km *kmmModule) SetNodeVersionLabelAsDesired(ctx context.Context, nwConfig *amdv1alpha1.NetworkConfig, nodes *v1.NodeList) error {
	// for each selected node
	// put the KMM version label given by CR's driver version
	// KMM operator will watch on the version label and manage the kmod upgrade
	labelKey, labelVal := GetVersionLabelKV(nwConfig)
	logger := log.FromContext(ctx)
	for _, node := range nodes.Items {
		if _, ok := node.Labels[labelKey]; ok {
			// version label was already put on the node object
			// our operator should only upload the version label for 0->1 installation
			// for 1->2 upgrade, we expect users to manually update the version label on Node resource to trigger ordered upgrade
			// so if thee label was already there, controller won't update it
			continue
		}
		if labelVal == "" {
			defaultVersion, err := utils.GetDefaultDriversVersion(node)
			if err != nil {
				logger.Error(err, fmt.Sprintf("failed to get default version for node %+v err %+v", node.GetName(), err))
			}
			labelVal = defaultVersion
		}
		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]string{
					labelKey: labelVal,
				},
			},
		}
		patchBytes, err := json.Marshal(patch)
		if err != nil {
			return fmt.Errorf("failed to marshal node label patch: %+v", err)
		}
		rawPatch := client.RawPatch(types.StrategicMergePatchType, patchBytes)
		if err := km.client.Patch(ctx, &node, rawPatch); err != nil {
			return fmt.Errorf("failed to patch node label: %+v", err)
		}
	}
	return nil
}

func (km *kmmModule) SetBuildConfigMapAsDesired(buildCM *v1.ConfigMap, nwConfig *amdv1alpha1.NetworkConfig) error {
	if buildCM.Data == nil {
		buildCM.Data = make(map[string]string)
	}
	if km.isOpenShift {
		buildCM.Data["dockerfile"] = buildOcDockerfile
	} else {
		dockerfile, err := resolveDockerfile(buildCM.Name, nwConfig)
		if err != nil {
			return err
		}
		buildCM.Data["dockerfile"] = dockerfile
	}
	return controllerutil.SetControllerReference(nwConfig, buildCM, km.scheme)
}

var driverLabels = map[string]string{
	"20.04": "focal",
	"22.04": "jammy",
	"24.04": "noble",
}

func resolveDockerfile(cmName string, nwConfig *amdv1alpha1.NetworkConfig) (string, error) {
	splits := strings.SplitN(cmName, "-", 4)
	osDistro := splits[0]
	version := splits[1]
	var dockerfileTemplate string
	switch osDistro {
	case "ubuntu":
		dockerfileTemplate = dockerfileTemplateUbuntu
		driverLabel, present := driverLabels[version]
		if !present {
			return "", fmt.Errorf("invalid ubuntu version, expected to be one of %v", maps.Keys(driverLabels))
		}
		dockerfileTemplate = strings.Replace(dockerfileTemplate, "$$DRIVER_LABEL", driverLabel, -1)

		// trigger to pull the internal ROCM dev build
		if internalArtifactoryURL, ok := os.LookupEnv("INTERNAL_ARTIFACTORY"); ok &&
			strings.Contains(strings.ToLower(nwConfig.Spec.Driver.AMDNetworkInstallerRepoURL), internalArtifactoryURL) {
			dockerfileTemplate = dockerfileDevTemplateUbuntu
			devBuildinfo := strings.Split(nwConfig.Spec.Driver.AMDNetworkInstallerRepoURL, " ")
			if len(devBuildinfo) < 4 {
				return "", fmt.Errorf("please provide internal build info, required 4 items: artifactory URL, installer deb file name, amdionic build number and rocm build tag, got: %+v", nwConfig.Spec.Driver.AMDNetworkInstallerRepoURL)
			}
			nwConfig.Spec.Driver.AMDNetworkInstallerRepoURL = devBuildinfo[0]
			dockerfileTemplate = strings.Replace(dockerfileTemplate, "$$DEV_DEB", devBuildinfo[1], -1)
			dockerfileTemplate = strings.Replace(dockerfileTemplate, "$$AMDNetwork_BUILD", devBuildinfo[2], -1)
			dockerfileTemplate = strings.Replace(dockerfileTemplate, "$$ROCM_BUILD", devBuildinfo[3], -1)
		}
		// use an environment variable to ask CI infra to pull image from internal repository
		// in order to avoid docekrhub pull rate limit issue
		_, isCIEnvSet := os.LookupEnv("CI_ENV")
		internalUbuntuBaseImage, internalUbuntuBaseSet := os.LookupEnv("INTERNAL_UBUNTU_BASE")
		if isCIEnvSet && internalUbuntuBaseSet {
			dockerfileTemplate = strings.Replace(dockerfileTemplate, "ubuntu:$$VERSION", fmt.Sprintf("%v:$$VERSION", internalUbuntuBaseImage), -1)
		}
	case "coreos":
		dockerfileTemplate = buildOcDockerfile
	// FIX ME
	// add the RHEL back when it is fully supported
	/*case "rhel":
	dockerfileTemplate = dockerfileTemplateRHEL
	versionSplits := strings.Split(version, ".")
	dockerfileTemplate = strings.Replace(dockerfileTemplate, "$$MAJOR_VERSION", versionSplits[0], -1)
	if nwConfig.Spec.RedhatSubscriptionUsername == "" || nwConfig.Spec.RedhatSubscriptionPassword == "" {
		return "", fmt.Errorf("Redhat subscription RedhatSubscriptionUsername and RedhatSubscriptionPassword required")
	}
	dockerfileTemplate = strings.Replace(dockerfileTemplate, "$$REDHAT_SUBSCRIPTION_USERNAME", nwConfig.Spec.RedhatSubscriptionUsername, -1)
	dockerfileTemplate = strings.Replace(dockerfileTemplate, "$$REDHAT_SUBSCRIPTION_PASSWORD", nwConfig.Spec.RedhatSubscriptionPassword, -1)
	*/
	default:
		return "", fmt.Errorf("not supported OS: %s", osDistro)
	}
	resolvedDockerfile := strings.Replace(dockerfileTemplate, "$$VERSION", version, -1)
	return resolvedDockerfile, nil
}

func (km *kmmModule) SetKMMModuleAsDesired(ctx context.Context, mod *kmmv1beta1.Module, nwConfig *amdv1alpha1.NetworkConfig, nodes *v1.NodeList) error {
	err := setKMMModuleLoader(ctx, mod, nwConfig, km.isOpenShift, nodes)
	if err != nil {
		return fmt.Errorf("failed to set KMM Module: %v", err)
	}
	return controllerutil.SetControllerReference(nwConfig, mod, km.scheme)
}

func setKMMModuleLoader(ctx context.Context, mod *kmmv1beta1.Module, nwConfig *amdv1alpha1.NetworkConfig, isOpenshift bool, nodes *v1.NodeList) error {
	kmlog := log.FromContext(ctx)
	kmlog.Info(fmt.Sprintf("isOpenshift %+v", isOpenshift))

	kernelMappings, driversVersion, err := getKernelMappings(nwConfig, isOpenshift, nodes)
	if err != nil {
		return err
	}

	mod.Spec.ModuleLoader.Container = kmmv1beta1.ModuleLoaderContainerSpec{
		Modprobe: kmmv1beta1.ModprobeSpec{
			ModuleName: networkDriverModuleName,
			Args:       &kmmv1beta1.ModprobeArgs{},
			ModulesLoadingOrder: []string{
				networkDriverModuleName,
				pdsCoreModuleName,
				tawkIPCModuleName,
			},
		},
		Version:        nwConfig.Spec.Driver.Version,
		KernelMappings: kernelMappings,
	}
	if mod.Spec.ModuleLoader.Container.Version == "" {
		mod.Spec.ModuleLoader.Container.Version = driversVersion
	}
	mod.Spec.ModuleLoader.ServiceAccountName = "amd-network-operator-kmm-module-loader"
	mod.Spec.ImageRepoSecret = nwConfig.Spec.Driver.ImageRegistrySecret
	mod.Spec.Selector = getNodeSelector(nwConfig)
	mod.Spec.Tolerations = append(nwConfig.Spec.Driver.Tolerations,
		v1.Toleration{
			Key:      "amd-network-driver-upgrade",
			Value:    "true",
			Operator: v1.TolerationOpEqual,
			Effect:   v1.TaintEffectNoSchedule,
		})
	return nil
}

func getKernelMappings(nwConfig *amdv1alpha1.NetworkConfig, isOpenshift bool, nodes *v1.NodeList) ([]kmmv1beta1.KernelMapping, string, error) {

	inTreeModuleToRemove := ""

	if nodes == nil || len(nodes.Items) == 0 {
		return nil, "", fmt.Errorf("No nodes found for the label selector %s", MapToLabelSelector(nwConfig.Spec.Selector))
	}
	kernelMappings := []kmmv1beta1.KernelMapping{}
	kmSet := map[string]bool{}
	var driversVersion string
	for _, node := range nodes.Items {
		km, ver, err := getKM(nwConfig, node, inTreeModuleToRemove, isOpenshift)
		if err != nil {
			return nil, driversVersion, fmt.Errorf("error constructing a kernel mapping for node: %s, err: %v", node.Name, err)
		}
		if kmSet[km.Literal] {
			continue
		}
		kernelMappings = append(kernelMappings, km)
		kmSet[km.Literal] = true
		driversVersion = ver
	}
	return kernelMappings, driversVersion, nil
}

func getKM(nwConfig *amdv1alpha1.NetworkConfig, node v1.Node, inTreeModuleToRemove string, isOpenShift bool) (kmmv1beta1.KernelMapping, string, error) {
	driversVersion := nwConfig.Spec.Driver.Version
	driversImage := nwConfig.Spec.Driver.Image
	var err error
	osName, err := GetOSName(node, nwConfig)
	if err != nil {
		return kmmv1beta1.KernelMapping{}, "", err
	}

	if isOpenShift {
		if driversVersion == "" {
			driversVersion = defaultOcDriversVersion
		}
		if driversImage == "" {
			driversImage = defaultOcDriversImageTemplate
		}
		driversImage = addNodeInfoSuffixToImageTag(driversImage, osName, driversVersion)
	} else {
		if driversVersion == "" {
			driversVersion, err = utils.GetDefaultDriversVersion(node)
			if err != nil {
				return kmmv1beta1.KernelMapping{}, "", err
			}
		}
		if driversImage == "" {
			driversImage = defaultDriversImageTemplate
		}
		driversImage = addNodeInfoSuffixToImageTag(driversImage, osName, driversVersion)
	}

	repoURL := defaultInstallerRepoURL
	if nwConfig.Spec.Driver.AMDNetworkInstallerRepoURL != "" {
		repoURL = nwConfig.Spec.Driver.AMDNetworkInstallerRepoURL
	}

	var registryTLS *kmmv1beta1.TLSOptions
	if (nwConfig.Spec.Driver.ImageRegistryTLS.Insecure != nil && *nwConfig.Spec.Driver.ImageRegistryTLS.Insecure) ||
		(nwConfig.Spec.Driver.ImageRegistryTLS.InsecureSkipTLSVerify != nil && *nwConfig.Spec.Driver.ImageRegistryTLS.InsecureSkipTLSVerify) {
		registryTLS = &kmmv1beta1.TLSOptions{}
		if nwConfig.Spec.Driver.ImageRegistryTLS.Insecure != nil {
			registryTLS.Insecure = *nwConfig.Spec.Driver.ImageRegistryTLS.Insecure
		}
		if nwConfig.Spec.Driver.ImageRegistryTLS.InsecureSkipTLSVerify != nil {
			registryTLS.InsecureSkipTLSVerify = *nwConfig.Spec.Driver.ImageRegistryTLS.InsecureSkipTLSVerify
		}
	}

	var kmmSign *kmmv1beta1.Sign
	if nwConfig.Spec.Driver.ImageSign.KeySecret != nil &&
		nwConfig.Spec.Driver.ImageSign.CertSecret != nil {
		kmmSign = &kmmv1beta1.Sign{
			KeySecret:   nwConfig.Spec.Driver.ImageSign.KeySecret,
			CertSecret:  nwConfig.Spec.Driver.ImageSign.CertSecret,
			FilesToSign: getKmodsToSign(isOpenShift, node.Status.NodeInfo.KernelVersion),
		}
		if registryTLS != nil {
			kmmSign.UnsignedImageRegistryTLS = *registryTLS
		}
	}

	kmmBuild := &kmmv1beta1.Build{
		DockerfileConfigMap: &v1.LocalObjectReference{
			Name: GetCMName(osName, nwConfig),
		},
		BuildArgs: []kmmv1beta1.BuildArg{
			{
				Name:  "DRIVERS_VERSION",
				Value: driversVersion,
			},
			{
				Name:  "REPO_URL",
				Value: repoURL,
			},
		},
	}

	_, isCIEnvSet := os.LookupEnv("CI_ENV")
	if isCIEnvSet {
		kmmBuild.BaseImageRegistryTLS.Insecure = true
		kmmBuild.BaseImageRegistryTLS.InsecureSkipTLSVerify = true
	}

	return kmmv1beta1.KernelMapping{
		Literal:              node.Status.NodeInfo.KernelVersion,
		ContainerImage:       driversImage,
		InTreeModuleToRemove: inTreeModuleToRemove,
		Build:                kmmBuild,
		Sign:                 kmmSign,
		RegistryTLS:          registryTLS,
	}, driversVersion, nil
}

func addNodeInfoSuffixToImageTag(imgStr string, osName, driversVersion string) string {
	// KMM will render and fulfill the value of ${KERNEL_FULL_VERSION}
	tag := osName + "-${KERNEL_FULL_VERSION}-" + driversVersion
	// tag cannot be more than 128 chars
	if len(tag) > 128 {
		tag = tag[len(tag)-128:]
	}
	return imgStr + ":" + tag
}

func GetCMName(osName string, nwCfg *amdv1alpha1.NetworkConfig) string {
	return osName + "-" + nwCfg.Name + "-" + nwCfg.Namespace
}

func GetOSName(node v1.Node, nwCfg *amdv1alpha1.NetworkConfig) (string, error) {
	osImageStr := strings.ToLower(node.Status.NodeInfo.OSImage)

	// sort the key of cmNameMappers
	// make sure in the given OS string, coreos was checked before all other types of RHEL string
	keys := make([]string, 0, len(cmNameMappers))
	for key := range cmNameMappers {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, os := range keys {
		if strings.Contains(osImageStr, os) {
			return cmNameMappers[os](osImageStr), nil
		}
	}

	return "", fmt.Errorf("OS: %s not supported. Should be one of %v", osImageStr, maps.Keys(cmNameMappers))
}

var cmNameMappers = map[string]func(fullImageStr string) string{
	"ubuntu":  ubuntuCMNameMapper,
	"coreos":  rhelCoreOSNameMapper,
	"rhel":    rhelCMNameMapper,
	"red hat": rhelCMNameMapper,
	"redhat":  rhelCMNameMapper,
}

func rhelCMNameMapper(osImageStr string) string {
	// Check if the input contains "Red Hat Enterprise Linux"
	// Use regex to find the release version
	re := regexp.MustCompile(`(\d+\.\d+)`)
	matches := re.FindStringSubmatch(osImageStr)
	if len(matches) > 1 {
		return fmt.Sprintf("%s-%s", "rhel", matches[1])
	}
	return "rhel-" + osImageStr
}

func rhelCoreOSNameMapper(osImageStr string) string {
	// Check if the input contains "Red Hat Enterprise Linux"
	// Use regex to find the release version
	re := regexp.MustCompile(`(\d+\.\d+)`)
	matches := re.FindStringSubmatch(osImageStr)
	if len(matches) > 1 {
		return fmt.Sprintf("%s-%s", "coreos", matches[1])
	}
	return "coreos-" + osImageStr
}

func ubuntuCMNameMapper(osImageStr string) string {
	splits := strings.Split(osImageStr, " ")
	os := splits[0]
	version := splits[1]
	versionSplits := strings.Split(version, ".")
	trimmedVersion := strings.Join(versionSplits[:2], ".")
	return fmt.Sprintf("%s-%s", os, trimmedVersion)
}

func GetK8SNodes(ls string) (*v1.NodeList, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	options := metav1.ListOptions{
		LabelSelector: ls,
	}
	return clientset.CoreV1().Nodes().List(context.TODO(), options)
}

func MapToLabelSelector(selector map[string]string) string {
	selectorSlice := make([]string, 0)
	for k, v := range selector {
		selectorSlice = append(selectorSlice, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(selectorSlice, ",")
}

func GetVersionLabelKV(nwConfig *amdv1alpha1.NetworkConfig) (string, string) {
	return fmt.Sprintf(kmmNodeVersionLabelTemplate, nwConfig.Namespace, nwConfig.Name), nwConfig.Spec.Driver.Version
}

func getNodeSelector(nwConfig *amdv1alpha1.NetworkConfig) map[string]string {
	if nwConfig.Spec.Selector != nil {
		return nwConfig.Spec.Selector
	}

	ns := make(map[string]string, 0)
	ns[utils.NodeFeatureLabelAmdNic] = "true"
	return ns
}

func getKmodsToSign(isOpenShift bool, kernelVersion string) []string {
	if isOpenShift {
		return []string{} // TODO add support for signing in OpenShift
	}
	return []string{
		"/opt/lib/modules/" + kernelVersion + "/updates/dkms/ib_peer_mem.ko",
		"/opt/lib/modules/" + kernelVersion + "/updates/dkms/ionic.ko",
		"/opt/lib/modules/" + kernelVersion + "/updates/dkms/ionic_rdma.ko",
		"/opt/lib/modules/" + kernelVersion + "/kernel/drivers/infiniband/core/ib_core.ko",
		"/opt/lib/modules/" + kernelVersion + "/kernel/drivers/infiniband/core/ib_uverbs.ko",
	}
}
