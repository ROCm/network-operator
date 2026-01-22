ifneq ("$(wildcard ../../dev.env)","")
    include ../../dev.env
endif

E2E_INIT_CONTAINER_IMAGE ?= registry.test.pensando.io:5000/busybox:1.36
E2E_METRICS_QUERY_CLIENT_IMAGE ?= curlimages/curl:7.78.0
E2E_GENERAL_UTILS_IMAGE ?= registry.test.pensando.io:5000/network-operator-utils:v1.1.0
E2E_UBUNTU_BASE_IMAGE ?= ubuntu:22.04
E2E_MINIO_IMAGE ?= minio/minio:latest
E2E_EXPORTER_IMAGE ?= registry.test.pensando.io:5000/device-metrics-exporter:latest
E2E_EXPORTER_IMAGE_MOCK ?= registry.test.pensando.io:5000/device-metrics-exporter-mock:latest
E2E_EXPORTER_IMAGE_2 ?= registry.test.pensando.io:5000/device-metrics-exporter/exporter-mock:v2
E2E_DEVICE_PLUGIN_IMAGE ?= registry.test.pensando.io:5000/k8s-network-device-plugin:v0.1
E2E_DEVICE_PLUGIN_IMAGE_2 ?= registry.test.pensando.io:5000/k8s-network-device-plugin:v0.1
E2E_NODE_LABELLER_IMAGE ?= rocm/k8s-device-plugin:labeller-latest
E2E_NODE_LABELLER_IMAGE_2 ?= rocm/k8s-device-plugin:labeller-1.31.0.6
E2E_TEST_RUNNER_IMAGE ?= rocm/test-runner:v1.2.0-beta.0
E2E_CNI_PLUGINS_IMAGE ?= rocm/k8s-cni-plugins:v0.1
E2E_CNI_PLUGINS_IMAGE_2 ?= rocm/k8s-cni-plugins:v0.1.1

export E2E_INIT_CONTAINER_IMAGE
export E2E_METRICS_QUERY_CLIENT_IMAGE
export E2E_GENERAL_UTILS_IMAGE
export E2E_UBUNTU_BASE_IMAGE
export E2E_MINIO_IMAGE
export E2E_EXPORTER_IMAGE
export E2E_EXPORTER_IMAGE_MOCK
export E2E_EXPORTER_IMAGE_2
export E2E_DEVICE_PLUGIN_IMAGE
export E2E_NODE_LABELLER_IMAGE
export E2E_DEVICE_PLUGIN_IMAGE_2
export E2E_NODE_LABELLER_IMAGE_2
export E2E_TEST_RUNNER_IMAGE
export E2E_CNI_PLUGINS_IMAGE
export E2E_CNI_PLUGINS_IMAGE_2

export E2E_DCM_IMAGE
export E2E_NODEAPP_IMG
export E2E_DRIVER_IMAGE_REPO

.DEFAULT: all
.PHONY: all lint

TEST_ARGS ?= ""
ifdef OPENSHIFT
	TEST_ARGS += "-openshift"
endif

ifdef SIM_ENABLE
	TEST_ARGS += "-simEnable"
endif

ifdef CI_ENV
	TEST_ARGS += "-ciEnv"
endif

all:
	go test -test.timeout=360m -v --count=1 $(TEST_ARGS);
lint:
	@go fmt ./...
	@goimports -w ./
	@go vet ./...
