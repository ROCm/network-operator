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

package client

import (
	"context"
	"encoding/json"

	"github.com/ROCm/network-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type ClientInterface interface {
	NetworkConfigs(namespace string) NetworkConfigsInterface
}

type NetworkConfigClient struct {
	restClient rest.Interface
}

func Client(c *rest.Config) (*NetworkConfigClient, error) {
	config := *c
	config.ContentConfig.GroupVersion = &v1alpha1.GroupVersion
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()

	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	return &NetworkConfigClient{restClient: client}, nil
}

func (c *NetworkConfigClient) NetworkConfigs(namespace string) NetworkConfigsInterface {
	return &networkConfigsClient{
		restClient: c.restClient,
		ns:         namespace,
	}
}

type networkConfigsClient struct {
	restClient rest.Interface
	ns         string
}

type NetworkConfigsInterface interface {
	Create(config *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error)
	Update(config *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error)
	List(opts metav1.ListOptions) (*v1alpha1.NetworkConfigList, error)
	PatchTestRunnerEnablement(config *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error)
	PatchTestRunnerConfigmap(config *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error)
	PatchMetricsExporterEnablement(config *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error)
	PatchMetricsExporter(config *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error)
	PatchDriversBlacklist(config *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error)
	PatchDriversVersion(config *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error)
	PatchUpgradePolicy(config *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error)
	PatchDevicePluginImage(config *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error)
	PatchNodeLabellerImage(config *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error)
	PatchMetricsExporterImage(config *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error)
	PatchCNIPluginsImage(config *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error)
	PatchMetricsExporterServiceType(config *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error)
	Get(name string, options metav1.GetOptions) (*v1alpha1.NetworkConfig, error)
	Delete(name string) (*v1alpha1.NetworkConfig, error)
}

func (c *networkConfigsClient) List(opts metav1.ListOptions) (*v1alpha1.NetworkConfigList, error) {
	result := v1alpha1.NetworkConfigList{}
	err := c.restClient.
		Get().
		Namespace(c.ns).
		Resource("networkConfigs").
		//VersionedParams(&opts, scheme.ParameterCodec).
		Do(context.TODO()).
		Into(&result)

	return &result, err
}

func (c *networkConfigsClient) Get(name string, opts metav1.GetOptions) (*v1alpha1.NetworkConfig, error) {
	result := v1alpha1.NetworkConfig{}
	err := c.restClient.
		Get().
		Namespace(c.ns).
		Resource("networkConfigs").
		Name(name).
		//VersionedParams(&opts, scheme.ParameterCodec).
		Do(context.TODO()).
		Into(&result)

	return &result, err
}

func (c *networkConfigsClient) Create(networkCfg *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error) {
	result := v1alpha1.NetworkConfig{}
	networkCfg.TypeMeta = metav1.TypeMeta{
		Kind:       "NetworkConfig",
		APIVersion: "amd.com/v1alpha1",
	}
	err := c.restClient.
		Post().
		Namespace(c.ns).
		Resource("networkConfigs").
		Body(networkCfg).
		Do(context.TODO()).
		Into(&result)

	return &result, err
}

func (c *networkConfigsClient) Update(networkCfg *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error) {
	result := v1alpha1.NetworkConfig{}
	networkCfg.TypeMeta = metav1.TypeMeta{
		Kind:       "NetworkConfig",
		APIVersion: "amd.com/v1alpha1",
	}
	err := c.restClient.
		Put().
		Namespace(networkCfg.Namespace).
		Resource("networkConfigs").
		Name(networkCfg.Name).
		Body(networkCfg).
		Do(context.TODO()).
		Into(&result)

	return &result, err
}

func (c *networkConfigsClient) PatchTestRunnerEnablement(networkCfg *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error) {
	result := v1alpha1.NetworkConfig{}
	networkCfg.TypeMeta = metav1.TypeMeta{
		Kind:       "NetworkConfig",
		APIVersion: "amd.com/v1alpha1",
	}

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"testRunner": map[string]bool{
				"enable": *networkCfg.Spec.TestRunner.Enable,
			},
		},
	}
	patchBytes, _ := json.Marshal(patch)

	err := c.restClient.
		Patch(types.MergePatchType).
		Namespace(networkCfg.Namespace).
		Resource("networkConfigs").
		Name(networkCfg.Name).
		Body(patchBytes).
		Do(context.TODO()).
		Into(&result)

	return &result, err
}

func (c *networkConfigsClient) PatchTestRunnerConfigmap(networkCfg *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error) {
	result := v1alpha1.NetworkConfig{}
	networkCfg.TypeMeta = metav1.TypeMeta{
		Kind:       "NetworkConfig",
		APIVersion: "amd.com/v1alpha1",
	}

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"testRunner": map[string]interface{}{
				"config": map[string]string{
					"name": networkCfg.Spec.TestRunner.Config.Name,
				},
			},
		},
	}
	patchBytes, _ := json.Marshal(patch)

	err := c.restClient.
		Patch(types.MergePatchType).
		Namespace(networkCfg.Namespace).
		Resource("networkConfigs").
		Name(networkCfg.Name).
		Body(patchBytes).
		Do(context.TODO()).
		Into(&result)

	return &result, err
}

func (c *networkConfigsClient) PatchMetricsExporterEnablement(networkCfg *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error) {
	result := v1alpha1.NetworkConfig{}
	networkCfg.TypeMeta = metav1.TypeMeta{
		Kind:       "NetworkConfig",
		APIVersion: "amd.com/v1alpha1",
	}

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"metricsExporter": map[string]bool{
				"enable": *networkCfg.Spec.MetricsExporter.Enable,
			},
		},
	}
	patchBytes, _ := json.Marshal(patch)

	err := c.restClient.
		Patch(types.MergePatchType).
		Namespace(networkCfg.Namespace).
		Resource("networkConfigs").
		Name(networkCfg.Name).
		Body(patchBytes).
		Do(context.TODO()).
		Into(&result)

	return &result, err
}

func (c *networkConfigsClient) PatchDriversBlacklist(networkCfg *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error) {
	result := v1alpha1.NetworkConfig{}
	networkCfg.TypeMeta = metav1.TypeMeta{
		Kind:       "NetworkConfig",
		APIVersion: "amd.com/v1alpha1",
	}

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"driver": map[string]interface{}{
				"blacklist": *networkCfg.Spec.Driver.Blacklist,
			},
			"devicePlugin": map[string]interface{}{
				"enableNodeLabeller": *networkCfg.Spec.DevicePlugin.EnableNodeLabeller,
			},
		},
	}
	patchBytes, _ := json.Marshal(patch)

	err := c.restClient.
		Patch(types.MergePatchType).
		Namespace(networkCfg.Namespace).
		Resource("networkConfigs").
		Name(networkCfg.Name).
		Body(patchBytes).
		Do(context.TODO()).
		Into(&result)

	return &result, err
}

func (c *networkConfigsClient) PatchDriversVersion(networkCfg *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error) {
	result := v1alpha1.NetworkConfig{}
	networkCfg.TypeMeta = metav1.TypeMeta{
		Kind:       "NetworkConfig",
		APIVersion: "amd.com/v1alpha1",
	}

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"driver": map[string]string{
				"version": networkCfg.Spec.Driver.Version,
			},
		},
	}
	patchBytes, _ := json.Marshal(patch)

	err := c.restClient.
		Patch(types.MergePatchType).
		Namespace(networkCfg.Namespace).
		Resource("networkConfigs").
		Name(networkCfg.Name).
		Body(patchBytes).
		Do(context.TODO()).
		Into(&result)

	return &result, err
}

func (c *networkConfigsClient) PatchUpgradePolicy(networkCfg *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error) {
	result := v1alpha1.NetworkConfig{}
	networkCfg.TypeMeta = metav1.TypeMeta{
		Kind:       "NetworkConfig",
		APIVersion: "amd.com/v1alpha1",
	}

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"driver": map[string]interface{}{
				"upgradePolicy": networkCfg.Spec.Driver.UpgradePolicy,
			},
		},
	}
	patchBytes, _ := json.Marshal(patch)

	err := c.restClient.
		Patch(types.MergePatchType).
		Namespace(networkCfg.Namespace).
		Resource("networkConfigs").
		Name(networkCfg.Name).
		Body(patchBytes).
		Do(context.TODO()).
		Into(&result)

	return &result, err
}

func (c *networkConfigsClient) PatchDevicePluginImage(networkCfg *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error) {
	result := v1alpha1.NetworkConfig{}
	networkCfg.TypeMeta = metav1.TypeMeta{
		Kind:       "NetworkConfig",
		APIVersion: "amd.com/v1alpha1",
	}

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"devicePlugin": map[string]string{
				"devicePluginImage": networkCfg.Spec.DevicePlugin.DevicePluginImage,
			},
		},
	}
	patchBytes, _ := json.Marshal(patch)

	err := c.restClient.
		Patch(types.MergePatchType).
		Namespace(networkCfg.Namespace).
		Resource("networkConfigs").
		Name(networkCfg.Name).
		Body(patchBytes).
		Do(context.TODO()).
		Into(&result)

	return &result, err
}

func (c *networkConfigsClient) PatchNodeLabellerImage(networkCfg *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error) {
	result := v1alpha1.NetworkConfig{}
	networkCfg.TypeMeta = metav1.TypeMeta{
		Kind:       "NetworkConfig",
		APIVersion: "amd.com/v1alpha1",
	}

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"devicePlugin": map[string]string{
				"nodeLabellerImage": networkCfg.Spec.DevicePlugin.NodeLabellerImage,
			},
		},
	}
	patchBytes, _ := json.Marshal(patch)

	err := c.restClient.
		Patch(types.MergePatchType).
		Namespace(networkCfg.Namespace).
		Resource("networkConfigs").
		Name(networkCfg.Name).
		Body(patchBytes).
		Do(context.TODO()).
		Into(&result)

	return &result, err
}

func (c *networkConfigsClient) PatchMetricsExporterImage(networkCfg *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error) {
	result := v1alpha1.NetworkConfig{}
	networkCfg.TypeMeta = metav1.TypeMeta{
		Kind:       "NetworkConfig",
		APIVersion: "amd.com/v1alpha1",
	}

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"metricsExporter": map[string]string{
				"image": networkCfg.Spec.MetricsExporter.Image,
			},
		},
	}
	patchBytes, _ := json.Marshal(patch)

	err := c.restClient.
		Patch(types.MergePatchType).
		Namespace(networkCfg.Namespace).
		Resource("networkConfigs").
		Name(networkCfg.Name).
		Body(patchBytes).
		Do(context.TODO()).
		Into(&result)

	return &result, err
}

func (c *networkConfigsClient) PatchCNIPluginsImage(networkCfg *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error) {
	result := v1alpha1.NetworkConfig{}
	networkCfg.TypeMeta = metav1.TypeMeta{
		Kind:       "NetworkConfig",
		APIVersion: "amd.com/v1alpha1",
	}

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"secondaryNetwork": map[string]interface{}{
				"cniPlugins": map[string]string{
					"image": networkCfg.Spec.SecondaryNetwork.CniPlugins.Image,
				},
			},
		},
	}
	patchBytes, _ := json.Marshal(patch)

	err := c.restClient.
		Patch(types.MergePatchType).
		Namespace(networkCfg.Namespace).
		Resource("networkConfigs").
		Name(networkCfg.Name).
		Body(patchBytes).
		Do(context.TODO()).
		Into(&result)

	return &result, err
}

func (c *networkConfigsClient) PatchMetricsExporterServiceType(networkCfg *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error) {
	result := v1alpha1.NetworkConfig{}
	networkCfg.TypeMeta = metav1.TypeMeta{
		Kind:       "NetworkConfig",
		APIVersion: "amd.com/v1alpha1",
	}

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"metricsExporter": map[string]string{
				"serviceType": string(networkCfg.Spec.MetricsExporter.SvcType),
			},
		},
	}
	patchBytes, _ := json.Marshal(patch)

	err := c.restClient.
		Patch(types.MergePatchType).
		Namespace(networkCfg.Namespace).
		Resource("networkConfigs").
		Name(networkCfg.Name).
		Body(patchBytes).
		Do(context.TODO()).
		Into(&result)

	return &result, err
}

func (c *networkConfigsClient) PatchMetricsExporter(networkCfg *v1alpha1.NetworkConfig) (*v1alpha1.NetworkConfig, error) {
	result := v1alpha1.NetworkConfig{}
	networkCfg.TypeMeta = metav1.TypeMeta{
		Kind:       "NetworkConfig",
		APIVersion: "amd.com/v1alpha1",
	}

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"metricsExporter": networkCfg.Spec.MetricsExporter,
		},
	}
	patchBytes, _ := json.Marshal(patch)

	err := c.restClient.
		Patch(types.MergePatchType).
		Namespace(networkCfg.Namespace).
		Resource("networkConfigs").
		Name(networkCfg.Name).
		Body(patchBytes).
		Do(context.TODO()).
		Into(&result)

	return &result, err
}

func (c *networkConfigsClient) Delete(name string) (*v1alpha1.NetworkConfig, error) {
	result := v1alpha1.NetworkConfig{}
	err := c.restClient.
		Delete().
		Namespace(c.ns).
		Resource("networkConfigs").
		Body(&v1alpha1.NetworkConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}).
		Do(context.TODO()).
		Into(&result)

	return &result, err
}
