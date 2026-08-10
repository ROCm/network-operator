#!/bin/bash

# Usage: ./docker-build.sh <ainic_version>
#
# All other options are configurable via environment variables with sensible defaults.
#
# Examples:
#   ./docker-build.sh 1.117.5-a-77
#   GPU_TARGETS=gfx942 ./docker-build.sh 1.117.5-a-77

set -euo pipefail

if [ $# -ne 1 ]; then
    echo "Usage: $0 <ainic_version>" >&2
    echo "" >&2
    echo "Environment variables (all optional):" >&2
    echo "  IMAGE_TAG         Image tag (auto-generated if not set)" >&2
    echo "  ROCM_BASE_IMAGE   ROCm base image (default: docker.io/rocm/dev-ubuntu-24.04:7.2)" >&2
    echo "  GPU_TARGETS       GPU architectures (default: gfx90a;gfx942;gfx950)" >&2
    echo "  RCCL_TAG          RCCL git branch/tag/SHA (default: rocm-7.2.0)" >&2
    echo "  ANP_TAG           ANP git tag/SHA (default: v1.3.0)" >&2
    echo "  UCX_VERSION       UCX git tag (default: v1.18.0)" >&2
    echo "  OMPI_VERSION      OpenMPI git tag (default: v4.1.6)" >&2
    echo "  RCCL_TESTS_SHA    RCCL tests commit (default: 78968d60...)" >&2
    exit 1
fi

IMAGE_NAME="roce-workload"
AINIC_VERSION="$1"
GPU_TARGETS="${GPU_TARGETS:-gfx90a;gfx942;gfx950}"
REPO_URL="https://repo.radeon.com"
RCCL_TAG="${RCCL_TAG:-rocm-7.2.0}"
ANP_TAG="${ANP_TAG:-v1.3.0}"
UCX_VERSION="${UCX_VERSION:-v1.18.0}"
OMPI_VERSION="${OMPI_VERSION:-v4.1.6}"
RCCL_TESTS_SHA="${RCCL_TESTS_SHA:-78968d60dbec47761ac398b5f26bb4e0ccb4db53}"

ROCM_BASE_IMAGE="${ROCM_BASE_IMAGE:-docker.io/rocm/dev-ubuntu-24.04:7.2}"
ROCM_VER="${ROCM_BASE_IMAGE##*:}"
ROCM_VER="${ROCM_VER:-unknown}"

IMAGE_TAG="${IMAGE_TAG:-ubuntu24_rocm-${ROCM_VER}_rccl-${RCCL_TAG}_anp-${ANP_TAG}_ainic-${AINIC_VERSION}}"

echo "Building ${IMAGE_NAME}:${IMAGE_TAG}"
echo "  ROCm:  ${ROCM_BASE_IMAGE}"
echo "  RCCL:  ${RCCL_TAG}"
echo "  ANP:   ${ANP_TAG}"
echo "  UCX:   ${UCX_VERSION}"
echo "  OMPI:  ${OMPI_VERSION}"
echo "  AINIC: ${AINIC_VERSION}"
echo "  GPUs:  ${GPU_TARGETS}"
echo ""

docker build \
    --build-arg ROCM_BASE_IMAGE="${ROCM_BASE_IMAGE}" \
    --build-arg REPO_URL="${REPO_URL}" \
    --build-arg AINIC_VERSION="${AINIC_VERSION}" \
    --build-arg IMAGE_NAME="${IMAGE_NAME}" \
    --build-arg IMAGE_TAG="${IMAGE_TAG}" \
    --build-arg GPU_TARGETS="${GPU_TARGETS}" \
    --build-arg RCCL_TAG="${RCCL_TAG}" \
    --build-arg RCCL_TESTS_SHA="${RCCL_TESTS_SHA}" \
    --build-arg ANP_TAG="${ANP_TAG}" \
    --build-arg UCX_VERSION="${UCX_VERSION}" \
    --build-arg OMPI_VERSION="${OMPI_VERSION}" \
    -t "${IMAGE_NAME}:${IMAGE_TAG}" \
    -f Dockerfile .

if command -v jq &>/dev/null; then
    docker inspect "${IMAGE_NAME}:${IMAGE_TAG}" --format '{{ json .Config.Labels }}' | jq
else
    docker inspect "${IMAGE_NAME}:${IMAGE_TAG}" --format '{{ json .Config.Labels }}'
fi
