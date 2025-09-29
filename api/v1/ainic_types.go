/*
Copyright 2024 Advanced Micro Devices, Inc.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AINICSpec defines the desired state of AINIC
type AINICSpec struct {
	// NodeSelector specifies the nodes where AINIC resources should be configured
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Driver specifies the AMD AINIC driver configuration
	Driver DriverSpec `json:"driver"`

	// NetworkConfig specifies network-related configurations
	NetworkConfig NetworkConfigSpec `json:"networkConfig,omitempty"`

	// Resources specifies resource allocation for AINIC
	Resources ResourcesSpec `json:"resources,omitempty"`
}

// DriverSpec defines the AMD AINIC driver configuration
type DriverSpec struct {
	// Image specifies the driver container image
	Image string `json:"image"`

	// Version specifies the driver version
	Version string `json:"version"`

	// Args specifies additional arguments for the driver
	Args []string `json:"args,omitempty"`

	// Env specifies environment variables for the driver
	Env []EnvVar `json:"env,omitempty"`
}

// NetworkConfigSpec defines network configuration for AINIC
type NetworkConfigSpec struct {
	// NetworkMode specifies the network mode (e.g., "SR-IOV", "DPDK")
	NetworkMode string `json:"networkMode,omitempty"`

	// VFs specifies the number of Virtual Functions to create
	VFs int32 `json:"vfs,omitempty"`

	// MTU specifies the Maximum Transmission Unit
	MTU int32 `json:"mtu,omitempty"`

	// VLAN specifies VLAN configuration
	VLAN []VLANConfig `json:"vlan,omitempty"`
}

// VLANConfig defines VLAN configuration
type VLANConfig struct {
	// ID specifies the VLAN ID
	ID int32 `json:"id"`

	// Priority specifies the VLAN priority
	Priority int32 `json:"priority,omitempty"`
}

// ResourcesSpec defines resource allocation
type ResourcesSpec struct {
	// Memory specifies memory allocation
	Memory string `json:"memory,omitempty"`

	// CPU specifies CPU allocation
	CPU string `json:"cpu,omitempty"`

	// HugepagesSize specifies hugepages size
	HugepagesSize string `json:"hugepagesSize,omitempty"`

	// HugepagesCount specifies hugepages count
	HugepagesCount int32 `json:"hugepagesCount,omitempty"`
}

// EnvVar represents an environment variable
type EnvVar struct {
	// Name of the environment variable
	Name string `json:"name"`

	// Value of the environment variable
	Value string `json:"value"`
}

// AINICStatus defines the observed state of AINIC
type AINICStatus struct {
	// Phase represents the current phase of AINIC deployment
	Phase string `json:"phase,omitempty"`

	// Conditions represents the latest available observations of AINIC state
	Conditions []AINICCondition `json:"conditions,omitempty"`

	// NodesReady represents the number of nodes with AINIC ready
	NodesReady int32 `json:"nodesReady,omitempty"`

	// NodesTotal represents the total number of nodes targeted
	NodesTotal int32 `json:"nodesTotal,omitempty"`

	// Message provides human-readable message about the current state
	Message string `json:"message,omitempty"`
}

// AINICCondition describes the state of AINIC at a certain point
type AINICCondition struct {
	// Type of AINIC condition
	Type string `json:"type"`

	// Status of the condition
	Status metav1.ConditionStatus `json:"status"`

	// LastTransitionTime is the last time the condition transitioned
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`

	// Reason is a unique, one-word, CamelCase reason for the condition's last transition
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable message indicating details about last transition
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="Nodes Ready",type=integer,JSONPath=`.status.nodesReady`
//+kubebuilder:printcolumn:name="Nodes Total",type=integer,JSONPath=`.status.nodesTotal`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AINIC is the Schema for the ainics API
type AINIC struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AINICSpec   `json:"spec,omitempty"`
	Status AINICStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AINICList contains a list of AINIC
type AINICList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AINIC `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AINIC{}, &AINICList{})
}
