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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAINICDefault(t *testing.T) {
	ainic := &AINIC{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ainic",
			Namespace: "default",
		},
		Spec: AINICSpec{
			Driver: DriverSpec{
				Image:   "amd/ainic-driver:latest",
				Version: "1.0.0",
			},
		},
	}

	if ainic.Name != "test-ainic" {
		t.Errorf("Expected name 'test-ainic', got %s", ainic.Name)
	}

	if ainic.Spec.Driver.Image != "amd/ainic-driver:latest" {
		t.Errorf("Expected image 'amd/ainic-driver:latest', got %s", ainic.Spec.Driver.Image)
	}
}

func TestNetworkConfigValidation(t *testing.T) {
	config := NetworkConfigSpec{
		NetworkMode: "SR-IOV",
		VFs:         8,
		MTU:         9000,
		VLAN: []VLANConfig{
			{ID: 100, Priority: 1},
			{ID: 200, Priority: 2},
		},
	}

	if config.NetworkMode != "SR-IOV" {
		t.Errorf("Expected network mode 'SR-IOV', got %s", config.NetworkMode)
	}

	if config.VFs != 8 {
		t.Errorf("Expected 8 VFs, got %d", config.VFs)
	}

	if len(config.VLAN) != 2 {
		t.Errorf("Expected 2 VLAN configs, got %d", len(config.VLAN))
	}
}
