ifneq ("$(wildcard ../../dev.env)","")
    include ../../dev.env
endif

export NETWORK_OPERATOR_CHART
export E2E_GPU_OP_HELM_CHART

.DEFAULT: all
.PHONY: all lint

TEST_ARGS ?= ""
ifdef SIM_ENABLE
	TEST_ARGS += "-simEnable"
endif

all:
	go test -test.timeout=360m -v --count=1 $(TEST_ARGS);

lint:
	@go fmt ./...
	@goimports -w ./
	@go vet ./...