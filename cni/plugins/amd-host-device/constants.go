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

package main

import "time"

const (
	// CNIPluginName is the name of the CNI plugin
	CNIPluginName = "amd-host-device"

	// file has the details of the interface names and it's IP mapping
	mappingsFilePath = "/var/lib/cni/amd-host-device/ip-interface-mappings.json"

	// defaultCNIPluginPath is the default path where CNI plugins are installed on the host
	defaultCNIPluginPath = "/opt/cni/bin"

	// pluginExecTimeout is the timeout for plugin execution (host-device)
	pluginExecTimeout = 5 * time.Minute

	// AMDHostDeviceCNILocalStore is the path where the CNI plugin stores its local state
	// This is used to store the IP and interface mappings
	AMDHostDeviceCNILocalStore = "/var/lib/cni/amd-host-device"
)
