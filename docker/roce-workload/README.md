# RoCE Workload Docker Image

RoCE/RCCL workload image bundling ROCm, RCCL, RCCL-Tests, AMD ANP, UCX, OpenMPI, and AINIC user-space drivers. This image is used for running distributed RCCL performance tests across AMD GPU clusters with AMD AINIC (Pollara) NICs.

Pre-built images are available on [Docker Hub](https://hub.docker.com/r/rocm/roce-workload).

**Note:** The workload image must be compatible with the AINIC driver version on the host nodes. Use an image tagged with the same AINIC version as your deployed drivers.

## Table of Contents

1. [Components](#components)
2. [Build Requirements](#build-requirements)
3. [Build](#build)
4. [Configuration](#configuration)
5. [Included Tools](#included-tools)
6. [Running and Deploying](#running-and-deploying)

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

## Running and Deploying

For deployment instructions (pod manifests, RCCL tests, CVF integration), see the [RoCE Workload Image documentation](https://instinct.docs.amd.com/projects/network-operator/en/latest/cluster_validation_framework/roce-workload.html).
