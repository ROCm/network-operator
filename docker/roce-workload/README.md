# RoCE Workload Docker Image

RoCE/RCCL workload image bundling ROCm, RCCL, RCCL-Tests, AMD ANP, UCX, OpenMPI, and AINIC user-space drivers. This image is used for running distributed RCCL performance tests across AMD GPU clusters with AMD AINIC (Pollara) NICs.

Pre-built images are available on [Docker Hub](https://hub.docker.com/r/rocm/roce-workload).

**Note:** The workload image must be compatible with the AINIC driver version on the host nodes. Use an image tagged with the same AINIC version as your deployed drivers.

## Table of Contents

1. [Components](#components)
2. [Build Requirements](#build-requirements)
3. [Build](#build)
4. [Configuration](#configuration)
5. [Verify the Image](#verify-the-image)
6. [Included Tools](#included-tools)
7. [Running RCCL Tests Manually](#running-rccl-tests-manually)
8. [Usage with CVF](#usage-with-cvf)

---

## Components

| Component | Source | Version control |
|---|---|---|
| ROCm | Base image | `ROCM_BASE_IMAGE` env var |
| UCX | git clone (openucx/ucx) | `UCX_VERSION` in Dockerfile (default: v1.18.0) |
| OpenMPI | git clone (open-mpi/ompi) | `OMPI_VERSION` in Dockerfile (default: v4.1.6) |
| RCCL | git clone (ROCm/rccl) | `RCCL_TAG` env var |
| RCCL Tests | git sparse-checkout (ROCm/rocm-systems) | `RCCL_TESTS_SHA` env var |
| AMD ANP | git clone (ROCm/amd-anp) | `ANP_TAG` env var |
| AINIC drivers | apt repo ([repo.radeon.com](https://repo.radeon.com/amdainic/pensando/ubuntu/)) | `AINIC_VERSION` (arg to docker-build.sh) |

---

## Build Requirements

Any machine with Docker, internet access, and at least 16 cores / 32 GB RAM / 50 GB disk. All dependencies are fetched from public sources (GitHub, Docker Hub, repo.radeon.com). Expect 30-60 minutes for a clean build with all GPU targets; subsequent builds reuse cached layers and are much faster.

---

## Build

```bash
./docker-build.sh <ainic_version>

# Auto-generates tag from component versions:
./docker-build.sh 1.117.5-a-77

# Override any option via env vars:
IMAGE_TAG=custom-tag ./docker-build.sh 1.117.5-a-77
ROCM_BASE_IMAGE=docker.io/rocm/dev-ubuntu-24.04:7.2 ./docker-build.sh 1.117.5-a-77
GPU_TARGETS=gfx942 ./docker-build.sh 1.117.5-a-77
```

**Auto-generated tag format:** `ubuntu<ver>_rocm-<ver>_rccl-<branch>_anp-<ver>_ainic-<ver>`

---

## Configuration

All options are configurable via environment variables:

| Variable | Default | Description |
|---|---|---|
| `IMAGE_TAG` | auto-generated | Image tag |
| `ROCM_BASE_IMAGE` | `docker.io/rocm/dev-ubuntu-24.04:7.2` | ROCm base image |
| `GPU_TARGETS` | `gfx90a;gfx942;gfx950` | GPU target architectures |
| `RCCL_TAG` | `rocm-7.2.0` | RCCL git branch/tag/SHA ([ROCm/rccl](https://github.com/ROCm/rccl)) |
| `ANP_TAG` | `v1.3.0` | ANP git tag/SHA ([ROCm/amd-anp](https://github.com/ROCm/amd-anp)) |
| `UCX_VERSION` | `v1.18.0` | UCX git tag ([openucx/ucx](https://github.com/openucx/ucx)) |
| `OMPI_VERSION` | `v4.1.6` | OpenMPI git tag ([open-mpi/ompi](https://github.com/open-mpi/ompi)) |
| `RCCL_TESTS_SHA` | `78968d60...` | RCCL tests commit ([ROCm/rocm-systems](https://github.com/ROCm/rocm-systems)) |

GPU target architectures:

| GPU target | Hardware |
|---|---|
| `gfx90a` | MI210, MI250 |
| `gfx942` | MI300X, MI325X |
| `gfx950` | MI450 |

To reduce build time, set only the targets you need (e.g., `GPU_TARGETS="gfx942"` for MI300X only).

---

## Verify the Image

After building, verify all components are present (find your tag with `docker images | grep roce-workload`):

```bash
docker run --rm roce-workload:<tag> bash -c '
  echo "RCCL:"; ls /root/rccl/build/release/librccl.so* | head -1
  echo "ANP:"; find / -name "librccl-anp.so" 2>/dev/null | head -1
  echo "RCCL tests:"; ls /root/rccl-tests/build/all_reduce_perf
  echo "MPI:"; /root/ompi/install/bin/mpirun --version | head -1
  echo "UCX:"; /root/ucx_build/bin/ucx_info -v | head -1
  echo "show_gid:"; ls /usr/sbin/show_gid
'
```

---

## Included Tools

- **`run_rccl.sh`** — standalone 2-node RCCL test runner. Configurable via environment variables:

  | Variable | Default | Description |
  |---|---|---|
  | `GPUS_PER_NODE` | 8 | GPUs per node |
  | `NUM_NODES` | 2 | Number of nodes |
  | `START_MSG_SIZE` | 1K | Starting message size |
  | `END_MSG_SIZE` | 1G | Ending message size |
  | `STEP_FACTOR` | 2 | Message size step multiplier |
  | `NUM_ITER` | 6 | Test iterations |
  | `NUM_WARMUP` | 20 | Warmup iterations |
  | `NCCL_DEBUG` | WARN | RCCL debug level (WARN, INFO, TRACE) |

- **`/usr/sbin/show_gid`** — display RDMA GID table

---

## Running RCCL Tests Manually

> This section is for quick validation that the image works correctly. For fleet-wide cluster validation, use the [Cluster Validation Framework (CVF)](#usage-with-cvf).

**Prerequisites:**

- [AMD Network Operator](https://instinct.docs.amd.com/projects/network-operator/en/main/) deployed with a `NetworkConfig` CR
- `amd-host-device-nad` [NetworkAttachmentDefinition](https://instinct.docs.amd.com/projects/network-operator/en/main/secondary_network/amd-host-device-cni.html) created
- [AMD GPU Operator](https://instinct.docs.amd.com/projects/gpu-operator/en/latest/) deployed (for `amd.com/gpu` device resources)

Each pod generates its own SSH key pair on startup. Before running MPI, exchange keys between pods:

### 1. Deploy workload pods

```bash
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: Pod
metadata:
  name: rccl-worker-0
  labels:
    app: rccl-test
  annotations:
    k8s.v1.cni.cncf.io/networks: amd-host-device-nad,amd-host-device-nad  # one entry per NIC
spec:
  restartPolicy: Never
  containers:
  - name: worker
    image: rocm/roce-workload:ubuntu24_rocm-7.2_rccl-ainic-oob-fb67e5b_anp-v1.3.0_ainic-1.117.5-a-77
    securityContext:
      capabilities:
        add: [IPC_LOCK]
    resources:
      requests:
        amd.com/gpu: 8   # adjust to match your hardware
        amd.com/nic: 2
      limits:
        amd.com/gpu: 8
        amd.com/nic: 2
    volumeMounts:
    - name: shm
      mountPath: /dev/shm
  volumes:
  - name: shm
    emptyDir:
      medium: Memory
      sizeLimit: 4Gi
---
apiVersion: v1
kind: Pod
metadata:
  name: rccl-worker-1
  labels:
    app: rccl-test
  annotations:
    k8s.v1.cni.cncf.io/networks: amd-host-device-nad,amd-host-device-nad  # one entry per NIC
spec:
  affinity:
    podAntiAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchLabels:
            app: rccl-test
        topologyKey: kubernetes.io/hostname
  restartPolicy: Never
  containers:
  - name: worker
    image: rocm/roce-workload:ubuntu24_rocm-7.2_rccl-ainic-oob-fb67e5b_anp-v1.3.0_ainic-1.117.5-a-77
    securityContext:
      capabilities:
        add: [IPC_LOCK]
    resources:
      requests:
        amd.com/gpu: 8
        amd.com/nic: 2
      limits:
        amd.com/gpu: 8
        amd.com/nic: 2
    volumeMounts:
    - name: shm
      mountPath: /dev/shm
  volumes:
  - name: shm
    emptyDir:
      medium: Memory
      sizeLimit: 4Gi
EOF
```

Adjust `amd.com/gpu` and `amd.com/nic` counts to match your hardware. The anti-affinity rule ensures pods land on different nodes automatically.

**For VMs with VNICs (SR-IOV VFs):** replace `amd.com/nic` with `amd.com/vnic` and `amd-host-device-nad` with `vf-amd-host-device-nad` in the pod spec above.

### 2. Exchange SSH keys

```bash
W0_KEY=$(kubectl exec rccl-worker-0 -- cat /root/.ssh/id_rsa.pub)
W1_KEY=$(kubectl exec rccl-worker-1 -- cat /root/.ssh/id_rsa.pub)
kubectl exec rccl-worker-0 -- bash -c "echo '$W1_KEY' >> /root/.ssh/authorized_keys"
kubectl exec rccl-worker-1 -- bash -c "echo '$W0_KEY' >> /root/.ssh/authorized_keys"
```

### 3. Run RCCL test

```bash
# Get pod IPs
W0_IP=$(kubectl get pod rccl-worker-0 -o jsonpath='{.status.podIP}')
W1_IP=$(kubectl get pod rccl-worker-1 -o jsonpath='{.status.podIP}')

# Run (GPUS_PER_NODE must match amd.com/gpu in the pod spec above)
kubectl exec rccl-worker-0 -- env GPUS_PER_NODE=8 run_rccl.sh $W0_IP $W1_IP all_reduce_perf
```

Available collectives: `all_reduce_perf`, `broadcast_perf`, `reduce_scatter_perf`, `all_gather_perf`, `alltoall_perf`, `reduce_perf`, `scatter_perf`, `gather_perf`, `sendrecv_perf`

A successful run prints a bandwidth table and ends with:

```text
# Avg bus bandwidth    : 6.66557
```

### 4. Cleanup

```bash
kubectl delete pod rccl-worker-0 rccl-worker-1
```

---

## Usage with CVF

Set the image in the [Cluster Validation Framework](https://instinct.docs.amd.com/projects/network-operator/en/main/cluster_validation_framework/README.html) `config.yaml`:

```yaml
RCCL_WORKLOAD_IMAGE: "rocm/roce-workload:ubuntu24_rocm-7.2_rccl-ainic-oob-fb67e5b_anp-v1.3.0_ainic-1.117.5-a-77"
```
