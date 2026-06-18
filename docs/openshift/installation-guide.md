# AMD Network Operator - Production Deployment Guide

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
   - [Infrastructure Requirements](#infrastructure-requirements)
   - [Required Operators Installation](#required-operators-installation)
   - [Development Tools (for building)](#development-tools-for-building)
3. [Architecture](#architecture)
4. [Cluster Configuration](#cluster-configuration)
5. [Installing the AMD Network Operator](#installing-the-amd-network-operator)
   - [Official Installation (Production)](#official-installation-production)
   - [Development Installation (Build from Source)](#development-installation-build-from-source)
6. [Post-Installation Verification](#post-installation-verification)
7. [Preparing Pre-Compiled Driver Images (Optional)](#preparing-pre-compiled-driver-images-optional)
   - [Two Driver Image Build Methods](#two-driver-image-build-methods)
   - [Method 1: RPM-based Build (Recommended)](#method-1-rpm-based-build-recommended)
   - [Method 2: Source Image Build (Advanced)](#method-2-source-image-build-advanced)
8. [Deploying NetworkConfig CR](#deploying-networkconfig-cr)
9. [Validation](#validation)
10. [Updating the Operator](#updating-the-operator)
11. [Cleanup](#cleanup)
12. [Troubleshooting](#troubleshooting)
13. [Key Implementation Details](#key-implementation-details)
14. [Production Checklist](#production-checklist)

---

## Overview

This guide provides production-ready steps for deploying the AMD Network Operator on OpenShift clusters using OLM (Operator Lifecycle Manager). This operator manages AMD network drivers (ionic, ionic_rdma, pds_core, tawk_ipc) using Kernel Module Management (KMM).

**What this operator does**:

- Automatically loads AMD network drivers on OpenShift CoreOS nodes
- Manages kernel module lifecycle through KMM
- Deploys device plugins for GPU-NIC integration
- Provides metrics and monitoring capabilities
- Supports RDMA and high-performance networking

### Quick Start Summary

**Time Required**: 30-45 minutes (excluding build time)

**High-Level Steps**:

1. Install NFD and KMM operators from OperatorHub *(5 min)*
2. Configure insecure registry (if needed) *(2 min)*
3. **Install AMD Network Operator**:
   - **Production**: Install from OperatorHub *(5 min)*
   - **Development**: Build and deploy from source *(15 min)*
4. Create NetworkConfig CR *(2 min)*
5. Verify drivers loaded on nodes *(5 min)*

> 💡 **Quick Start**: For connected environments, you can skip directly to step 4 after installing the operator. KMM will automatically build driver images in-cluster using the OpenShift internal registry.

**Key Requirements**:

- OpenShift 4.16+ with CoreOS
- NFD and KMM operators installed
- Container registry (insecure registry configured if internal)
- AMD Pensando NICs installed on nodes

---

## Important Notes

> **TIP**: This guide uses production-style versioning (`v1.0.0-netop-beta`). Replace with your actual version tags.

<!-- -->

> **WARNING**: Only install KMM operator **ONCE** in `openshift-kmm` namespace. Multiple instances cause conflicts.

<!-- -->

> **REGISTRY**: Configure insecure registries at cluster level before starting. Images won't pull otherwise.

---

## Prerequisites

### Infrastructure Requirements

- OpenShift 4.16+ cluster with CoreOS nodes
- AMD Pensando network hardware
- Container registry accessible from the cluster
- Administrative access to OpenShift cluster

### Required Operators Installation

**These operators must be installed BEFORE deploying the AMD Network Operator**:

#### 1. Install Node Feature Discovery (NFD)

NFD detects hardware features on nodes and labels them accordingly.

**Installation via OpenShift Web Console**:

1. Log in to OpenShift Web Console
2. Navigate to **Operators** → **OperatorHub**
3. Search for **"Node Feature Discovery"**
4. Click on the operator from Red Hat
5. Click **Install**
6. Keep default settings:
   - **Update Channel**: Select the latest stable channel
   - **Installation Mode**: All namespaces on the cluster
   - **Installed Namespace**: `openshift-nfd` (auto-created)
   - **Update Approval**: Automatic
7. Click **Install** and wait for the operator to become ready

**Verification**:

```bash
kubectl get csv -n openshift-nfd | grep nfd
# Expected: nfd.x.x.x    Node Feature Discovery    x.x.x    Succeeded
```

**Create a NodeFeatureDiscovery instance to activate NFD**:

After installing the NFD operator, create a `NodeFeatureDiscovery` CR to start NFD workers on the cluster:

1. Navigate to **Operators** → **Installed Operators** → **Node Feature Discovery**
2. Click the **NodeFeatureDiscovery** tab
3. Click **Create NodeFeatureDiscovery**
4. Accept defaults and click **Create**

Or via CLI:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: nfd.openshift.io/v1
kind: NodeFeatureDiscovery
metadata:
  name: nfd-instance
  namespace: openshift-nfd
spec:
  operand:
    image: quay.io/openshift/origin-node-feature-discovery:4.16
    servicePort: 12000
  workerConfig:
    configData: |
      core:
        sleepInterval: 60s
      sources:
        pci:
          deviceClassWhitelist:
            - "0200"
            - "03"
            - "12"
          deviceLabelFields:
            - "vendor"
            - "device"
EOF
```

```bash
# Verify NFD workers are running
kubectl get pods -n openshift-nfd | grep worker
# Expected: nfd-worker pods Running on each node
```

> **Note**: Without a `NodeFeatureDiscovery` instance, the NFD operator is installed but idle — no node feature detection or labeling occurs.

#### 2. Install Kernel Module Management (KMM)

KMM manages out-of-tree kernel modules on OpenShift clusters.

**Installation via OpenShift Web Console**:

1. Log in to OpenShift Web Console
2. Navigate to **Operators** → **OperatorHub**
3. Search for **"Kernel Module Management"**
4. Click on the operator from Red Hat
5. Click **Install**
6. Configure installation settings:
   - **Update Channel**: Select `stable` or latest channel
   - **Installation Mode**: All namespaces on the cluster
   - **Installed Namespace**: `openshift-kmm` (auto-created)
   - **Update Approval**: Automatic
7. Click **Install** and wait for the operator to become ready

**Verification**:

```bash
kubectl get csv -n openshift-kmm | grep kernel-module-management
# Expected: kernel-module-management.v2.5.1    Kernel Module Management    2.5.1    Succeeded

kubectl get deployment -n openshift-kmm
# Expected:
# NAME                      READY   UP-TO-DATE   AVAILABLE
# kmm-operator-controller   1/1     1            1
# kmm-operator-webhook      1/1     1            1
```

**⚠️ IMPORTANT**: Only install KMM **once** in the `openshift-kmm` namespace. Multiple KMM instances cause conflicts and module loading failures.

### Development Tools (for building)

- Docker or Podman
- Go 1.23+
- make
- operator-sdk v1.32.0+
- Git

## Architecture

```text
NetworkConfig CR → AMD Network Operator → KMM Module CR → KMM Operator → Driver Pods → Node (drivers loaded)
```

## Cluster Configuration

### 1. Configure Insecure Registry (if using internal registry)

OpenShift needs to trust your internal registry for pulling images without TLS:

```bash
# Check current configuration
kubectl get image.config.openshift.io/cluster -o yaml

# If your registry is not listed, add it:
kubectl patch image.config.openshift.io/cluster --type=merge \
  -p "{\"spec\":{\"registrySources\":{\"insecureRegistries\":[\"${REGISTRY_URL}\"]}}}"
```

**Note**: This configuration allows all nodes to pull from the specified registry without TLS verification.

### 2. Blacklist In-Tree Ionic Driver (Recommended)

OpenShift CoreOS ships with an in-tree `ionic` kernel module that loads at boot and can conflict with the out-of-tree driver installed by the AMD Network Operator. Apply a MachineConfig to blacklist the in-tree module and prevent it from loading at boot. The operator also configures KMM to remove any loaded in-tree module at runtime, but the blacklist prevents the brief window where the in-tree driver is active before KMM intervenes.

Create a `MachineConfig` to blacklist the in-tree `ionic` and `ionic_rdma` modules:

```yaml
apiVersion: machineconfiguration.openshift.io/v1
kind: MachineConfig
metadata:
  labels:
    machineconfiguration.openshift.io/role: worker
  name: ionic-module-blacklist
spec:
  config:
    ignition:
      version: 3.2.0
    storage:
      files:
        - path: "/etc/modprobe.d/ionic-blacklist.conf"
          mode: 0644
          overwrite: true
          contents:
            source: "data:text/plain;base64,YmxhY2tsaXN0IGlvbmljCmJsYWNrbGlzdCBpb25pY19yZG1hCg=="
```

Save the above manifest to a file and apply it:

```bash
oc apply -f ionic-module-blacklist.yaml
```

**Note**: Applying a `MachineConfig` will trigger a rolling reboot of the worker nodes managed by the Machine Config Operator (MCO). The base64 content decodes to:

```text
blacklist ionic
blacklist ionic_rdma
```

### 3. Verify KMM Installation

```bash
# Verify KMM is running in openshift-kmm namespace
kubectl get csv -n openshift-kmm | grep kernel-module-management

# Check KMM deployments
kubectl get deployment -n openshift-kmm
# Expected output:
# NAME                      READY   UP-TO-DATE   AVAILABLE
# kmm-operator-controller   1/1     1            1
# kmm-operator-webhook      1/1     1            1
```

**Troubleshooting**: If KMM exists in multiple namespaces, keep only the one in `openshift-kmm` to avoid conflicts.

### 4. Set Environment Variables

Set version variables that will be used throughout the deployment:

```bash
# Driver and firmware versions
export DRIVERS_VERSION="1.117.5-a-56"
export KERNEL_VERSION="5.14.0-570.76.1.el9_6.x86_64"
export RHEL_VERSION="9.6"

# Operator versions
export OPERATOR_VERSION="v1.0.0-netop-beta"

# Registry configuration
export REGISTRY_URL="registry.test.pensando.io:5000"
export REPO_URL="https://repo.radeon.com"

# DTK image (get from: kubectl get is -n openshift driver-toolkit -o yaml)
export DTK_IMAGE="quay.io/openshift-release-dev/ocp-v4.0-art-dev@sha256:288b3574a5524121c139b846e98a223da793305560f8b42dcd8d2aa712912998"
```

**Finding Version Values**:

```bash
# Get node kernel version
export KERNEL_VERSION=$(kubectl get nodes -o jsonpath='{.items[0].status.nodeInfo.kernelVersion}')

# Find matching DTK image
export DTK_IMAGE=$(kubectl get is -n openshift driver-toolkit -o jsonpath="{.spec.tags[?(@.name=='${KERNEL_VERSION}')].from.name}")
```

---

## Installing the AMD Network Operator

Choose the appropriate installation method based on your use case:

- **Official Installation**: Install published operator from OperatorHub (recommended for production)
- **Development Installation**: Build and install from source (for development and testing)

---

### Official Installation (Production)

This method installs the AMD Network Operator from Red Hat OperatorHub. Use this for production deployments when the operator is officially published.

#### Step 1: Install from OperatorHub via Web Console

1. Log in to **OpenShift Web Console**
2. Navigate to **Operators** → **OperatorHub**
3. Search for **"AMD Network Operator"**
4. Click on the **AMD Network Operator** tile
5. Click **Install**
6. Configure installation settings:
   - **Update Channel**: Select `stable` or latest channel
   - **Installation Mode**: Select namespace (e.g., `openshift-amd-network`)
   - **Installed Namespace**: `openshift-amd-network` (create if doesn't exist)
   - **Update Approval**: Automatic (recommended) or Manual
7. Click **Install** and wait for the operator to become ready

#### Step 2: Verify Installation

```bash
# Check operator is installed
kubectl get csv -n openshift-amd-network | grep amd-network-operator
# Expected: amd-network-operator.vX.Y.Z    AMD Network Operator    X.Y.Z    Succeeded

# Verify operator pod is running
kubectl get pods -n openshift-amd-network -l control-plane=controller-manager
# Expected: STATUS Running

# Check operator logs
kubectl logs -f deployment/amd-network-operator-controller-manager -n openshift-amd-network
```

#### Step 3: Proceed to Deployment

Once the operator is installed, proceed to [Deploying NetworkConfig CR](#deploying-networkconfig-cr) to start using the operator.

> 💡 **Optional**: If you need to pre-build driver images (for air-gapped environments or external registries), see [Preparing Pre-Compiled Driver Images](#preparing-pre-compiled-driver-images-optional).

---

### Development Installation (Build from Source)

This method is for developers and testers who need to build and deploy the operator from source code.

#### Prerequisites

Ensure you have the following tools installed:

- Docker or Podman
- Go 1.23+
- make
- operator-sdk v1.32.0+
- Git

#### Step 1: Build Operator Image

```bash
# Clone repository
git clone https://github.com/ROCm/network-operator.git
cd network-operator
git checkout <your-branch>

# Set image tags
export OPERATOR_IMG=${REGISTRY_URL}/amd-network-operator:${OPERATOR_VERSION}
export BUNDLE_IMG=${REGISTRY_URL}/amd-network-operator-bundle:${OPERATOR_VERSION}

# Build operator image
make docker-build IMG=${OPERATOR_IMG}

# Push to registry
docker push ${OPERATOR_IMG}
```

#### Step 2: Build OLM Bundle

The OLM bundle packages the operator for deployment via Operator Lifecycle Manager:

```bash
# Build bundle with your version
make bundle-build \
  IMG=${OPERATOR_IMG} \
  BUNDLE_IMG=${BUNDLE_IMG} \
  PROJECT_VERSION=${OPERATOR_VERSION}

# Push bundle image
make bundle-push BUNDLE_IMG=${BUNDLE_IMG}
```

**What this does**:

- Generates CSV (ClusterServiceVersion) with operator metadata
- Creates RBAC manifests for all service accounts
- Packages CRDs and required resources
- Builds and pushes a container image with the bundle

#### Step 3: Deploy via OLM

Deploy the bundle image using operator-sdk:

```bash
# Build the OLM bundle
make bundle-build

# Push bundle image to registry
make bundle-push

# Deploy using operator-sdk (automatically creates CatalogSource, Subscription, etc.)
./bin/operator-sdk run bundle ${BUNDLE_IMG} \
  --use-http \
  --skip-tls \
  -n openshift-amd-network
```

**Flags explained**:

- `--use-http`: Use HTTP instead of HTTPS for registry communication
- `--skip-tls`: Skip TLS verification (for insecure registries)
- `-n`: Target namespace for operator deployment

> 💡 **What `operator-sdk run bundle` does**: This command automatically creates the CatalogSource, OperatorGroup, and Subscription resources needed by OLM. You don't need to create them manually!

#### Step 4: Verify Installation

```bash
# Verify CSV is in Succeeded phase
kubectl get csv -n openshift-amd-network

# Check operator pod
kubectl get pods -n openshift-amd-network -l control-plane=controller-manager

# View operator logs
kubectl logs -f deployment/amd-network-operator-controller-manager \
  -n openshift-amd-network
```

---

## Post-Installation Verification

The operator creates multiple service accounts for different components:

```bash
kubectl get sa -n openshift-amd-network

# Expected service accounts:
# - amd-network-operator-controller-manager
# - amd-network-operator-device-plugin
# - amd-network-operator-kmm-module-loader
# - amd-network-operator-node-labeller
# - amd-network-operator-metrics-exporter
# - amd-network-operator-config-manager
# - amd-network-operator-utils-container
```

---

## Preparing Pre-Compiled Driver Images (Optional)

> ⚠️ **THIS SECTION IS OPTIONAL**: For most users with connected clusters, you can **skip this entire section** and proceed directly to [Deploying NetworkConfig CR](#deploying-networkconfig-cr). When you create a NetworkConfig CR, KMM will automatically build driver images in-cluster using the OpenShift internal registry.

**When to use this section**:

- **Air-gapped/disconnected environments**: No internet access during runtime
- **Pre-staging images**: Want driver images ready before deployment
- **External registry requirements**: Need images in a specific external registry
- **Custom build pipelines**: Integrating with CI/CD systems

**When to skip this section**:

- **Connected clusters**: Have internet access to `repo.radeon.com`
- **Quick start/trial**: Want the fastest path to running drivers
- **Using internal registry**: OpenShift's built-in registry is sufficient

> 💡 **WORKFLOW TIP**: If you do choose to pre-build images, you can do this in parallel while the operator deploys. The operator will wait idle until you create a NetworkConfig CR.

---

### Two Driver Image Build Methods

The operator supports two methods for building driver images, controlled by the `useSourceImage` field in NetworkConfig CR:

**Method 1: RPM-based Build** (`useSourceImage: false`) - **Recommended**:

- Downloads pre-compiled RPM packages from repo.radeon.com
- Installs drivers directly from RPMs
- Faster build process
- Uses: `DockerfileTemplate.rpm.ionic.coreos`

**Method 2: Source Image Build** (`useSourceImage: true`) - **Advanced**:

- Requires building a source image first containing driver source code
- KMM compiles modules from source against specific kernel
- More flexible for custom builds
- Uses: `DockerfileTemplate.srcimg.ionic.coreos` + source image from `internal-example/driverSrcImage/Dockerfile.ionic.coreos`

---

### Method 1: RPM-based Build (Recommended)

#### Step 1: Build Driver Image Using BuildConfig

**Prerequisites**: The BuildConfig requires a pull secret for accessing the Driver Toolkit (DTK) image from `quay.io`:

```bash
# The global-pull-secret typically contains credentials for:
# - quay.io (for DTK images)
# - registry.redhat.io (for UBI base images)
# - cloud.openshift.com (for OpenShift pull-through cache)

# Verify pull secret exists
kubectl get secret global-pull-secret -n openshift-amd-network

# If missing, copy from openshift-config or create new one
kubectl get secret pull-secret -n openshift-config -o yaml | \
  sed 's/namespace: openshift-config/namespace: openshift-amd-network/' | \
  sed 's/name: pull-secret/name: global-pull-secret/' | \
  kubectl apply -f -
```

This method downloads RPM packages and installs pre-compiled drivers:

```bash
cat > /tmp/driver-buildconfig.yaml << EOF
apiVersion: image.openshift.io/v1
kind: ImageStream
metadata:
  name: amdnetwork_kmod
  namespace: openshift-amd-network
spec:
  lookupPolicy:
    local: true
---
apiVersion: build.openshift.io/v1
kind: BuildConfig
metadata:
  name: amd-driver-build
  namespace: openshift-amd-network
spec:
  output:
    to:
      kind: ImageStreamTag
      name: amdnetwork_kmod:coreos-${RHEL_VERSION}-${KERNEL_VERSION}-${DRIVERS_VERSION}
  source:
    type: Git
    git:
      uri: "https://github.com/ROCm/network-operator.git"
      ref: <your_branch>
    contextDir: "internal/kmmmodule/dockerfiles"
    dockerfile: "DockerfileTemplate.rpm.ionic.coreos"
  strategy:
    type: Docker
    dockerStrategy:
      pullSecret:
        name: global-pull-secret
      buildArgs:
        - name: DTK_AUTO
          value: "${DTK_IMAGE}"
        - name: KERNEL_VERSION
          value: "${KERNEL_VERSION}"
        - name: DRIVERS_VERSION
          value: "${DRIVERS_VERSION}"
        - name: REPO_URL
          value: "${REPO_URL}"
      forcePull: true
  triggers: []
EOF

# Apply BuildConfig
kubectl apply -f /tmp/driver-buildconfig.yaml

# Start the build
kubectl start-build amd-driver-build -n openshift-amd-network

# Follow the build logs (this takes 10-15 minutes)
kubectl logs -f build/amd-driver-build-1 -n openshift-amd-network
```

**Build Arguments Explained**:

- `DTK_AUTO`: Driver Toolkit image matching your OpenShift version and kernel
- `KERNEL_VERSION`: Target kernel version from node
- `DRIVERS_VERSION`: AMD driver package version from repo.radeon.com
- `REPO_URL`: AMD repository URL

**Finding the Correct DTK Image**:

```bash
# Get node kernel version (already set in env vars)
echo $KERNEL_VERSION

# Find matching DTK image
kubectl get is -n openshift driver-toolkit -o yaml | grep "${KERNEL_VERSION}"
```

#### Step 4b: Push Driver Image to External Registry

The BuildConfig creates an ImageStream in OpenShift's internal registry. Push it to an external registry for KMM to access:

```bash
# Get the node IP
NODE_IP=<your-node-ip>

# SSH to the node
ssh core@${NODE_IP}

# Find the built image
sudo podman images | grep amdnetwork_kmod

# Tag with external registry
sudo podman tag <image-id> \
  ${REGISTRY_URL}/amdnetwork_kmod:coreos-${RHEL_VERSION}-${KERNEL_VERSION}-${DRIVERS_VERSION}

# Push to external registry
sudo podman push --tls-verify=false \
  ${REGISTRY_URL}/amdnetwork_kmod:coreos-${RHEL_VERSION}-${KERNEL_VERSION}-${DRIVERS_VERSION}
```

Why push to an external registry?

- OpenShift's internal registry may not be accessible during module loading
- External registry provides consistent access across cluster operations
- Simplifies image management and versioning

---

### Method 2: Source Image Build (Advanced)

This method first builds a source container image, then KMM compiles modules from that source against the specific kernel.

> **AIR-GAPPED ENVIRONMENTS**: This approach is designed for air-gapped or disconnected environments where direct access to external repositories (like `repo.radeon.com`) is restricted. By building a source image first, all required driver sources are packaged into a container that can be transferred and used in isolated environments without internet access during module compilation.

#### Step 5a: Use Pre-Built Source Images (Recommended)

Pre-built source images are available on Docker Hub for all published driver versions:

```bash
# Available at:
docker.io/amdpsdo/amdnic-drivers:<version>

# Example versions:
# docker.io/amdpsdo/amdnic-drivers:1.117.5-a-56
# docker.io/amdpsdo/amdnic-drivers:1.117.5
# docker.io/amdpsdo/amdnic-drivers:1.117.1
```

These images are automatically built and published by the GitHub Actions workflow. You can skip to [Step 5c](#step-5c-configure-networkconfig-to-use-source-image) and use these images directly.

#### Step 5a (Alternative): Build Source Image Manually

If you need to build source images yourself (e.g., for a custom driver version or internal registry):

**Option 1: Using the automated builder script**:

```bash
cd internal-example/driverSrcImage
./build-all-source-images.sh --version ${DRIVERS_VERSION} --registry your-registry.com
```

**Option 2: Using OpenShift BuildConfig**:

```bash
cat > /tmp/source-image-build.yaml << EOF
apiVersion: image.openshift.io/v1
kind: ImageStream
metadata:
  name: amdainic-driver-source
  namespace: openshift-amd-network
spec:
  lookupPolicy:
    local: true
---
apiVersion: build.openshift.io/v1
kind: BuildConfig
metadata:
  name: amd-source-image-build
  namespace: openshift-amd-network
spec:
  output:
    to:
      kind: ImageStreamTag
      name: amdainic-driver-source:latest
  source:
    type: Git
    git:
      uri: "https://github.com/ROCm/network-operator.git"
      ref: "main"
    contextDir: "internal-example/driverSrcImage"
    dockerfile: "Dockerfile.ionic.coreos"
  strategy:
    type: Docker
    dockerStrategy:
      buildArgs:
        - name: REPO_URL
          value: "${REPO_URL}"
        - name: MAJOR_VERSION
          value: "9"
        - name: DRIVERS_VERSION
          value: "${DRIVERS_VERSION}"
      forcePull: true
  triggers: []
EOF

kubectl apply -f /tmp/source-image-build.yaml
kubectl start-build amd-source-image-build -n openshift-amd-network
kubectl logs -f build/amd-source-image-build-1 -n openshift-amd-network
```

**What source images contain**:

- `/ionic_src/driver/` - Source code for ionic, pds, tawk-ipc modules
- `/ionic_src/firmware/` - Firmware files

#### Step 5b: Push Source Image to External Registry (If Built Manually)

Skip this step if using pre-built images from `docker.io/amdpsdo/amdnic-drivers`.

```bash
# SSH to a node
NODE_IP=<your-node-ip>
ssh core@${NODE_IP}

# Find and push the source image
sudo podman images | grep amdainic-driver-source

sudo podman tag <image-id> \
  ${REGISTRY_URL}/amdainic-driver-source:${DRIVERS_VERSION}

sudo podman push --tls-verify=false \
  ${REGISTRY_URL}/amdainic-driver-source:latest
```

#### Step 5c: Configure NetworkConfig to Use Source Image

When creating your NetworkConfig CR, set `useSourceImage: true` and provide the source image repository:

```yaml
spec:
  driver:
    enable: true
    useSourceImage: true  # Enable source image build
    version: "${DRIVERS_VERSION}"
    image: ${REGISTRY_URL}/amdnetwork_kmod  # Final driver image (compiled .ko files)
    imageBuild:
      sourceImageRepo: "docker.io/amdpsdo/amdnic-drivers"  # Pre-built source images
```

> 💡 **Note**: If using a custom/internal source image registry, replace `docker.io/amdpsdo/amdnic-drivers` with your registry path.

**How it works**:

1. KMM uses `DockerfileTemplate.srcimg.ionic.coreos`
2. Copies source code from your source image (`sourceImageRepo`)
3. Compiles modules against the Driver Toolkit (DTK) for the specific kernel version
4. Creates final driver image with compiled `.ko` files

---

## Verifying Service Accounts

The operator creates multiple service accounts for different components:

```bash
kubectl get sa -n openshift-amd-network

# Expected service accounts:
# - amd-network-operator-controller-manager
# - amd-network-operator-device-plugin
# - amd-network-operator-kmm-module-loader
# - amd-network-operator-node-labeller
# - amd-network-operator-metrics-exporter
# - amd-network-operator-config-manager
# - amd-network-operator-utils-container
```

## Deploying NetworkConfig CR

### 1. Create NFD Rule for NIC Detection

Create a `NodeFeatureRule` to instruct NFD to automatically label nodes that have AMD Pensando NICs:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeatureRule
metadata:
  name: amd-nic-label-nfd-rule
spec:
  rules:
  - name: amd-vnic
    labels:
      feature.node.kubernetes.io/amd-vnic: "true"
    matchAny:
      - matchFeatures:
          - feature: pci.device
            matchExpressions:
              vendor: {op: In, value: ["1dd8"]}
              device: {op: In, value: ["1003"]}
              subsystem_vendor: {op: In, value: ["1dd8"]}
              subsystem_device: {op: In, value: ["5201"]}
  - name: amd-nic
    labels:
      feature.node.kubernetes.io/amd-nic: "true"
    matchAny:
      - matchFeatures:
          - feature: pci.device
            matchExpressions:
              vendor: {op: In, value: ["1dd8"]}
              device: {op: In, value: ["1002"]}
              subsystem_vendor: {op: In, value: ["1dd8"]}
              subsystem_device: {op: In, value: ["5201"]}
EOF
```

Verify that nodes with AMD Pensando NICs are labeled:

```bash
kubectl get nodes -l feature.node.kubernetes.io/amd-nic=true
```

> **Note**: This requires the NFD operator to be installed and a `NodeFeatureDiscovery` CR to be created (see [Prerequisites](#required-operators-installation)). NFD will automatically apply the `feature.node.kubernetes.io/amd-nic: "true"` label to any node with AMD Pensando PCI devices (vendor `1dd8`). No node reboot is required.

### 2. Create NetworkConfig

Create the NetworkConfig CR to deploy drivers on your nodes. Choose the configuration based on which build method you used:

**Option 1: RPM-based Build** (if you used Method 1):

```bash
cat <<EOF | kubectl apply -f -
apiVersion: amd.com/v1alpha1
kind: NetworkConfig
metadata:
  name: amd-network
  namespace: openshift-amd-network
spec:
  selector:
    feature.node.kubernetes.io/amd-nic: "true"

  driver:
    enable: true
    version: "${DRIVERS_VERSION}"
    useSourceImage: false  # Use RPM-based build
    image: ${REGISTRY_URL}/amdnetwork_kmod
    imageRegistrySecret:
      name: global-pull-secret  # Optional: if registry requires auth
    imageRegistryTLS:
      insecure: true
      insecureSkipTLSVerify: true
    imageBuild:
      baseImageRegistryTLS:
        insecure: true
        insecureSkipTLSVerify: true
    AMDNetworkInstallerRepoURL: "${REPO_URL}"

  devicePlugin:
    enableNodeLabeller: true
    devicePluginImage: docker.io/rocm/k8s-device-plugin:rhubi-latest
    nodeLabellerImage: docker.io/rocm/k8s-device-plugin:labeller-rhubi-latest

  metricsExporter:
    enable: true
    image: docker.io/rocm/device-metrics-exporter:v1.2.0
EOF
```

**Option 2: Source Image Build** (for air-gapped environments or when using pre-built source images):

```bash
cat <<EOF | kubectl apply -f -
apiVersion: amd.com/v1alpha1
kind: NetworkConfig
metadata:
  name: amd-network
  namespace: openshift-amd-network
spec:
  selector:
    feature.node.kubernetes.io/amd-nic: "true"

  driver:
    enable: true
    version: "${DRIVERS_VERSION}"
    useSourceImage: true  # Use source image build
    image: ${REGISTRY_URL}/amdnetwork_kmod
    imageBuild:
      sourceImageRepo: "docker.io/amdpsdo/amdnic-drivers"  # Pre-built source images
      baseImageRegistryTLS:
        insecure: true
        insecureSkipTLSVerify: true
    imageRegistrySecret:
      name: global-pull-secret  # Optional: if registry requires auth
    imageRegistryTLS:
      insecure: true
      insecureSkipTLSVerify: true
    AMDNetworkInstallerRepoURL: "${REPO_URL}"

  devicePlugin:
    enableNodeLabeller: true
    devicePluginImage: docker.io/rocm/k8s-device-plugin:rhubi-latest
    nodeLabellerImage: docker.io/rocm/k8s-device-plugin:labeller-rhubi-latest

  metricsExporter:
    enable: true
    image: docker.io/rocm/device-metrics-exporter:v1.2.0
EOF
```

### 3. Monitor Deployment

```bash
# Watch NetworkConfig status
kubectl get networkconfig -n openshift-amd-network -w

# Check KMM Module creation
kubectl get module -n openshift-amd-network

# View Module status
kubectl get module amd-network -n openshift-amd-network -o yaml

# Check driver pods
kubectl get pods -n openshift-amd-network
```

## Validation

### Complete Validation Checklist

```bash
# 1. Operator Running
kubectl get pods -n openshift-amd-network -l control-plane=controller-manager
# Status: Running

# 2. NetworkConfig Applied
kubectl get networkconfig -n openshift-amd-network
# Status: Should show your config

# 3. KMM Module Created
kubectl get module -n openshift-amd-network
# Status: moduleLoader.nodesMatchingSelectorNumber should match node count

# 4. Device Plugin Running
kubectl get pods -n openshift-amd-network -l app=device-plugin
# Status: Running on target nodes

# 5. Node Labeller Running
kubectl get pods -n openshift-amd-network -l app=node-labeller
# Status: Running on target nodes

# 6. Drivers Loaded on Node
kubectl debug node/<node-name> -- chroot /host lsmod | grep -E '^(ionic|pds_core|tawk_ipc)'
# Expected: ionic, ionic_rdma, pds_core, tawk_ipc modules loaded

# 7. RDMA Devices Available
kubectl debug node/<node-name> -- chroot /host ls /sys/class/infiniband/
# Expected: ionic_0, ionic_1, ... (one per NIC)
```

## Updating the Operator

### Update to New Version

```bash
# Build new operator image
export NEW_VERSION=v1.0.1-netop-beta
export OPERATOR_IMG=${REGISTRY_URL}/amd-network-operator:${NEW_VERSION}
export BUNDLE_IMG=${REGISTRY_URL}/amd-network-operator-bundle:${NEW_VERSION}

make docker-build IMG=${OPERATOR_IMG}
docker push ${OPERATOR_IMG}

# Build new bundle
make bundle-build \
  IMG=${OPERATOR_IMG} \
  BUNDLE_IMG=${BUNDLE_IMG} \
  PROJECT_VERSION=${NEW_VERSION}

make bundle-push BUNDLE_IMG=${BUNDLE_IMG}

# Update via operator-sdk
./bin/operator-sdk run bundle-upgrade ${BUNDLE_IMG} \
  --use-http \
  --skip-tls \
  -n openshift-amd-network
```

## Cleanup

### Remove Operator Completely

```bash
# Delete all NetworkConfig CRs first
kubectl delete networkconfigs.amd.com -n openshift-amd-network --all

# Clean up using operator-sdk
./bin/operator-sdk cleanup amd-network-operator -n openshift-amd-network

# Or manually delete subscription and CSV
kubectl delete subscription amd-network-operator -n openshift-amd-network
kubectl delete csv amd-network-operator.<version> -n openshift-amd-network

# Delete CatalogSource (if using catalog method)
kubectl delete catalogsource amd-network-operator-catalog -n openshift-marketplace

# Delete namespace (optional)
kubectl delete namespace openshift-amd-network
```

## Troubleshooting

### Common Issues and Solutions

#### 1. operator-sdk: TLS Error with Insecure Registry

**Problem**:

```text
http: server gave HTTP response to HTTPS client
```

**Root Cause**: operator-sdk running locally doesn't know about cluster's insecure registry configuration.

**Solution**: Use `--use-http` and `--skip-tls` flags:

```bash
./bin/operator-sdk run bundle ${BUNDLE_IMG} --use-http --skip-tls -n <namespace>
```

---

#### 2. OperatorGroup Conflict

**Problem**:

```text
csv failed: reason: "InterOperatorGroupOwnerConflict"
intersecting operatorgroups provide the same apis
```

**Root Cause**: Another operator instance already exists providing the same CRDs.

**Solution**: Only one operator instance allowed per cluster:

1. Check existing operators: `kubectl get csv -A | grep amd-network`
2. Either use existing namespace or cleanup old deployment first
3. Delete test namespaces if created during troubleshooting

---

#### 3. Image Pull Failures

**Problem**: Pods stuck in `ImagePullBackOff` or `ErrImagePull`

**Root Cause**: Registry not accessible or missing credentials.

**Solution**:

1. **For insecure registries**: Verify configuration

   ```bash
   kubectl get image.config.openshift.io/cluster -o yaml | grep insecureRegistries
   ```

2. **For authenticated registries**: Check pull secrets

   ```bash
   kubectl get secret -n openshift-amd-network | grep pull
   ```

3. **Test registry access** from node:

   ```bash
   kubectl debug node/<node-name> -- chroot /host podman pull --tls-verify=false <image>
   ```

---

#### 4. Driver Modules Not Loading

**Problem**: `lsmod` shows no ionic modules on node

**Diagnostic Steps**:

```bash
# 1. Check KMM Module status
kubectl get module -n openshift-amd-network -o yaml

# 2. Check if worker pods ran
kubectl get pods -n openshift-amd-network | grep worker

# 3. Check worker pod logs
kubectl logs -n openshift-amd-network <kmm-worker-pod>

# 4. Verify node selector
kubectl get module -n openshift-amd-network -o jsonpath='{.spec.selector}'
kubectl get nodes --show-labels | grep <label>
```

**Common Causes**:

- Node selector doesn't match any nodes
- Driver image pull failed
- Module build failed (check BuildConfig logs)
- Module loading order incorrect (ionic must load before ionic_rdma)

---

#### 5. Wrong Module Loading Order

**Problem**: `ionic_rdma` fails to load with dependency errors

**Root Cause**: Module loading order is incorrect - `ionic` must load before `ionic_rdma`.

**Verification**:

```bash
kubectl get module -n openshift-amd-network -o jsonpath='{.spec.moduleLoader.container.modprobe}'
```

**Expected Output**:

```json
{
  "moduleName": "ionic",
  "modulesLoadingOrder": ["ionic", "ionic_rdma", "pds_core", "tawk_ipc"]
}
```

**Fix**: Ensure code in `internal/kmmmodule/kmmmodule.go` sets:

- `ModuleName: ionicModuleName` for OpenShift
- Correct loading order array

## Key Implementation Details

### Module Loading Order

The operator loads modules in this specific order (required for dependencies):

1. `ionic` - Base driver
2. `ionic_rdma` - RDMA support (depends on ionic)
3. `pds_core` - PDS core functionality
4. `tawk_ipc` - IPC support

**Code Location**: `internal/kmmmodule/kmmmodule.go`

- Uses `ModuleName: ionicModuleName` (OpenShift)
- Uses `ModuleName: networkDriverModuleName` (Ubuntu/K8s)

### Multus CNI Dependency

The device plugin has a hard dependency on Multus CNI:

- Waits for Multus config in init container
- Checks both `/etc/cni/net.d/` and `/etc/kubernetes/cni/net.d/` (OpenShift)
- Uses Multus device-info API at `/var/run/k8s.cni.cncf.io/devinfo/dp`

**Code Location**: `internal/deviceplugin/deviceplugin.go`

### Service Account Naming

All service accounts use consistent naming without platform-specific suffixes to maintain backward compatibility:

- `amd-network-operator-device-plugin` (not `-kmm-device-plugin`)
- Helm charts and RBAC must match these names

## Production Checklist

- [ ] KMM operator installed in `openshift-kmm` namespace only
- [ ] Insecure registry configured if using internal registry
- [ ] Operator image built and pushed with production tag (e.g., `v1.0.0-netop-beta`)
- [ ] Bundle image built and pushed with same version tag
- [ ] CatalogSource created and READY
- [ ] Subscription created with correct channel and source
- [ ] CSV in Succeeded phase
- [ ] NetworkConfig CR applied with correct node selector
- [ ] KMM Module created and modules loaded on target nodes
- [ ] Device plugin and node labeller pods running
- [ ] RDMA devices visible in `/sys/class/infiniband/`
- [ ] Validation tests passed

## References

- [AMD Network Operator Repository](https://github.com/ROCm/network-operator)
- [Kernel Module Management Documentation](https://openshift-kmm.netlify.app/)
- [OpenShift Operator Lifecycle Manager](https://docs.openshift.com/container-platform/latest/operators/understanding/olm/olm-understanding-olm.html)
- [Operator SDK Documentation](https://sdk.operatorframework.io/)

---

**Document Version**: 1.0.0<br>
**Target Platform**: OpenShift 4.16+ with CoreOS
