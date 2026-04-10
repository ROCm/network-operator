# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

AMD Network Operator is a Kubernetes operator that manages AMD AINIC networking components. It's built using the Kubebuilder framework and manages the lifecycle of device plugins, node labellers, CNI plugins, metrics exporters, and driver management via KMM (Kernel Module Management).

**Architecture**: Controller-based operator pattern with reconciliation loops managing `NetworkConfig` CRD. The operator coordinates multiple components: NFD (Node Feature Discovery), Multus CNI, KMM for driver management, device plugin, node labeller, and metrics exporter.

**Go version**: 1.23.0

### Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          Kubernetes Cluster                                  │
│                                                                              │
│  ┌────────────────────────────────────────────────────────────────────┐     │
│  │                    Control Plane / Master Node                      │     │
│  │                                                                      │     │
│  │  ┌──────────────────────────────────────────────────────────────┐  │     │
│  │  │        AMD Network Operator (Controller Manager)             │  │     │
│  │  │                                                                │  │     │
│  │  │  • Reconciles NetworkConfig CR                               │  │     │
│  │  │  • Manages component lifecycle                               │  │     │
│  │  │  • Coordinates upgrades                                       │  │     │
│  │  │  • Creates/updates DaemonSets, ConfigMaps, Services          │  │     │
│  │  └──────────────────────────────────────────────────────────────┘  │     │
│  │                              │                                       │     │
│  │                              │ watches/reconciles                    │     │
│  │                              ▼                                       │     │
│  │  ┌──────────────────────────────────────────────────────────────┐  │     │
│  │  │           NetworkConfig Custom Resource (CR)                 │  │     │
│  │  │                                                                │  │     │
│  │  │  spec:                                                         │  │     │
│  │  │    - devicePlugin config                                      │  │     │
│  │  │    - metricsExporter config                                   │  │     │
│  │  │    - secondaryNetwork config                                  │  │     │
│  │  │    - driver config (KMM Module)                               │  │     │
│  │  └──────────────────────────────────────────────────────────────┘  │     │
│  └────────────────────────────────────────────────────────────────────┘     │
│                                                                              │
│  ┌────────────────────────────────────────────────────────────────────┐     │
│  │                        Worker Nodes                                 │     │
│  │                                                                      │     │
│  │  ┌───────────────────────┐  ┌──────────────────────────────────┐  │     │
│  │  │ NFD (Node Feature     │  │  KMM (Kernel Module Mgmt)        │  │     │
│  │  │ Discovery)            │  │                                   │  │     │
│  │  │                       │  │  • Driver installation           │  │     │
│  │  │ • Detects AMD NICs    │  │  • Driver upgrades               │  │     │
│  │  │ • Labels nodes        │  │  • Module lifecycle              │  │     │
│  │  └───────────────────────┘  └──────────────────────────────────┘  │     │
│  │                                                                      │     │
│  │  ┌───────────────────────────────────────────────────────────────┐ │     │
│  │  │              DaemonSets (managed by operator)                 │ │     │
│  │  │                                                                │ │     │
│  │  │  ┌─────────────────┐  ┌──────────────────┐  ┌──────────────┐ │ │     │
│  │  │  │ Device Plugin   │  │ Node Labeller    │  │ Metrics      │ │ │     │
│  │  │  │                 │  │                  │  │ Exporter     │ │ │     │
│  │  │  │ • Registers     │  │ • Adds detailed  │  │              │ │ │     │
│  │  │  │   amd.com/nic   │  │   NIC labels     │  │ • Prometheus │ │ │     │
│  │  │  │ • Allocates     │  │ • PCI info       │  │   metrics    │ │ │     │
│  │  │  │   NICs to pods  │  │ • Capabilities   │  │ • Health     │ │ │     │
│  │  │  └─────────────────┘  └──────────────────┘  └──────────────┘ │ │     │
│  │  │                                                                │ │     │
│  │  │  ┌──────────────────────────────────────────────────────────┐ │ │     │
│  │  │  │         CNI Plugins (via Multus)                         │ │ │     │
│  │  │  │                                                            │ │ │     │
│  │  │  │  • host-device CNI                                        │ │ │     │
│  │  │  │  • amd-host-device CNI                                    │ │ │     │
│  │  │  │  • SR-IOV CNI                                             │ │ │     │
│  │  │  └──────────────────────────────────────────────────────────┘ │ │     │
│  │  └───────────────────────────────────────────────────────────────┘ │     │
│  │                                                                      │     │
│  │  ┌───────────────────────────────────────────────────────────────┐ │     │
│  │  │                    Application Pods                           │ │     │
│  │  │                                                                │ │     │
│  │  │  Pod Spec:                                                    │ │     │
│  │  │    resources:                                                 │ │     │
│  │  │      limits:                                                  │ │     │
│  │  │        amd.com/nic: 1     ← Requests NIC via Device Plugin   │ │     │
│  │  │    annotations:                                               │ │     │
│  │  │      k8s.v1.cni.cncf.io/networks: ainic-net  ← Secondary net │ │     │
│  │  └───────────────────────────────────────────────────────────────┘ │     │
│  └────────────────────────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────────────────────┘

Component Flow:
  1. NFD detects AMD NICs and labels nodes
  2. User creates NetworkConfig CR
  3. Operator reconciles and deploys DaemonSets
  4. KMM installs/manages drivers on nodes
  5. Device Plugin registers NIC resources
  6. Node Labeller adds detailed NIC metadata
  7. CNI Plugins prepare network configuration
  8. Pods request NICs and get secondary networks attached
  9. Metrics Exporter provides monitoring data
```

For the full visual diagram, see [docs/_static/amd-network-operator-diagram.jpg](docs/_static/amd-network-operator-diagram.jpg).

## Build and Test Commands

### Building

```bash
# Build manager binary
make manager

# Build Docker image (default tag: dev)
make docker-build

# Build everything (vendor, generate, manager, manifests, helm chart)
make all

# Skip vendor update (faster when dependencies unchanged)
make skip-vendor

# Build inside Docker container (includes full environment)
make default
```

### Testing

```bash
# Run unit tests with coverage
make unit-test

# Run specific unit tests
go test ./internal/controllers -v
go test ./internal/kmmmodule -v

# Run e2e tests (requires cluster and helm chart)
make e2e

# Run helm chart e2e tests
make helm-e2e

# Run DCM e2e tests
make dcm_e2e

# Lint code
make lint

# Format code
make fmt

# Vet code
make vet
```

### Development Workflow

```bash
# Generate manifests and CRDs after API changes
make manifests

# Generate DeepCopy methods and mocks after code changes
make generate

# Update Go dependencies
make vendor

# Update image registry URLs
make update-registry

# Update project version
make update-version
```

### Helm Operations

```bash
# Generate Kubernetes helm charts
make helm-k8s

# Generate OpenShift helm charts
OPENSHIFT=1 make helm-openshift

# Install operator via helm (creates kube-amd-network namespace)
make helm-install

# Uninstall operator via helm
make helm-uninstall

# Install/uninstall with options
SKIP_NFD=1 make helm-install       # Skip NFD deployment
SKIP_KMM=1 make helm-install       # Skip KMM deployment
SIM_ENABLE=1 make helm-install     # Enable simulation mode
```

### OpenShift Bundle Operations

```bash
# Build OLM bundle
make bundle-build

# Deploy bundle via OLM
make bundle-deploy

# Clean up OLM deployment
make bundle-cleanup
```

## Code Organization

### Core Components

- **`api/v1alpha1/`**: CRD definitions, primarily `NetworkConfig`
- **`cmd/main.go`**: Operator entrypoint, manager setup
- **`internal/controllers/`**: NetworkConfig controller with reconciliation logic
- **`internal/kmmmodule/`**: KMM Module management for driver installation
- **`internal/deviceplugin/`**: Device plugin lifecycle management
- **`internal/nodelabeller/`**: Node labelling logic
- **`internal/metricsexporter/`**: Metrics exporter management
- **`internal/secondarynetwork/`**: CNI plugins and Multus integration
- **`internal/configmanager/`**: Configuration management
- **`internal/workermgr/`**: Worker node management coordination

### Configuration and Deployment

- **`config/`**: Kustomize manifests for CRDs, RBAC, manager deployment
- **`helm-charts-k8s/`**: Kubernetes helm charts (generated)
- **`helm-charts-openshift/`**: OpenShift helm charts (generated)
- **`hack/`**: Build scripts, patches, and tooling
- **`bundle/`**: OLM bundle manifests

### Testing

- **`tests/e2e/`**: End-to-end tests using Ginkgo/Gomega
- **`tests/helm-e2e/`**: Helm chart validation tests
- **`internal/controllers/*_test.go`**: Unit tests with mocks

## Development Patterns

### Controller Development

The NetworkConfig controller follows standard Kubernetes controller-runtime patterns:
- Reconciliation loop processes NetworkConfig resources
- Uses conditions to track component states
- Manages dependent resources (DaemonSets, ConfigMaps, etc.)
- Leverages common-infra-operator package for shared components

### Testing Approach

- Unit tests use `go.uber.org/mock` for mocking interfaces
- E2e tests use Ginkgo BDD framework
- Mocks are generated via `make generate` using `//go:generate` directives
- Tests validate both positive and negative scenarios

### Image Management

Image references are injected at build time through Makefile variables:
- `DOCKER_REGISTRY`: Registry URL (default: `docker.io/rocm`)
- `IMAGE_TAG`: Image tag (default: `dev`)
- `PROJECT_VERSION`: Semantic version (default: `v0.0.1`)

Update image references across manifests with `make update-registry`.

### Versioning

When bumping version:
1. Set `PROJECT_VERSION` in Makefile or via environment variable
2. Run `make update-version` to update all manifests and Go code
3. Image tags in Go code are updated via sed patterns

## Platform Support

The operator supports both vanilla Kubernetes and OpenShift:
- **Kubernetes**: Uses `kubectl`, includes Multus subchart
- **OpenShift**: Uses `oc`, includes NFD and KMM subcharts
- Set `OPENSHIFT=1` environment variable to build/deploy for OpenShift

## External Dependencies

- **common-infra-operator**: Shared library at `./external/common-infra-operator` (git submodule)
- **KMM**: Kernel Module Management for driver lifecycle
- **NFD**: Node Feature Discovery for hardware detection
- **Multus**: CNI meta-plugin for secondary networks

Update submodules with `make submodule-all`.

## Container Runtime

Both Docker and Podman are supported. Set `CONTAINER_ENGINE=podman` to use Podman instead of Docker (default).

## Common Gotchas

- Always run `make vendor` after changing `go.mod`
- Run `make manifests` after modifying API types
- Run `make generate` after adding new `//go:generate` directives or changing interfaces
- Helm charts are generated artifacts - edit templates in `hack/` not in `helm-charts-*/`
- CRDs in helm charts are moved to `crds/` directory during build
- NetworkConfig must be deleted before uninstalling operator to ensure clean teardown
