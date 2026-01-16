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
	"log"
	"os"
)

var (
	exporterImage           string
	exporterImage2          string
	exporterImageMock       string
	devicePluginImage       string
	nodeLabellerImage       string
	devicePluginImage2      string
	nodeLabellerImage2      string
	cniPluginsImage         string
	cniPluginsImage2        string
	metricsQueryClientImage string
	generalUtilsImage       string
)

func init() {
	var ok bool
	// read e2e related env variables
	exporterImage, ok = os.LookupEnv("E2E_EXPORTER_IMAGE")
	if !ok {
		log.Fatalf("E2E_EXPORTER_IMAGE is not defined")
	}
	exporterImageMock, ok = os.LookupEnv("E2E_EXPORTER_IMAGE_MOCK")
	if !ok {
		log.Fatalf("E2E_EXPORTER_IMAGE_MOCK is not defined")
	}
	metricsQueryClientImage, ok = os.LookupEnv("E2E_METRICS_QUERY_CLIENT_IMAGE")
	if !ok {
		log.Fatalf("E2E_METRICS_QUERY_CLIENT_IMAGE is not defined")
	}
	exporterImage2, ok = os.LookupEnv("E2E_EXPORTER_IMAGE_2")
	if !ok {
		log.Fatalf("E2E_EXPORTER_IMAGE_2 is not defined")
	}
	devicePluginImage, ok = os.LookupEnv("E2E_DEVICE_PLUGIN_IMAGE")
	if !ok {
		log.Fatalf("E2E_DEVICE_PLUGIN_IMAGE is not defined")
	}
	nodeLabellerImage, ok = os.LookupEnv("E2E_NODE_LABELLER_IMAGE")
	if !ok {
		log.Fatalf("E2E_NODE_LABELLER_IMAGE is not defined")
	}
	devicePluginImage2, ok = os.LookupEnv("E2E_DEVICE_PLUGIN_IMAGE_2")
	if !ok {
		log.Fatalf("E2E_DEVICE_PLUGIN_IMAGE_2 is not defined")
	}
	nodeLabellerImage2, ok = os.LookupEnv("E2E_NODE_LABELLER_IMAGE_2")
	if !ok {
		log.Fatalf("E2E_NODE_LABELLER_IMAGE_2 is not defined")
	}
	cniPluginsImage, ok = os.LookupEnv("E2E_CNI_PLUGINS_IMAGE")
	if !ok {
		log.Fatalf("E2E_CNI_PLUGINS_IMAGE is not defined")
	}
	cniPluginsImage2, ok = os.LookupEnv("E2E_CNI_PLUGINS_IMAGE_2")
	if !ok {
		log.Fatalf("E2E_CNI_PLUGINS_IMAGE_2 is not defined")
	}
	generalUtilsImage, ok = os.LookupEnv("E2E_GENERAL_UTILS_IMAGE")
	if !ok {
		log.Fatalf("E2E_GENERAL_UTILS_IMAGE is not defined")
	}
}
