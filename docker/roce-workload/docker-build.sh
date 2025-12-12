#!/bin/bash

# Usage:         $0 	  	    <img_tag> 	  <ainic_repo_bundle>
#           ./docker-build.sh  	 tag_ver1  	   1.117.1-a-63

IMAGE_NAME="roce-workload"
IMAGE_VER="$1"
AMD_GPUS="gfx90a;gfx942;gfx950"
REPO_URL="https://repo.radeon.com"
DRIVER_LABEL="noble"
DRIVERS_VERSION="$2"
RCCL_DROP_TAG="drop/2025-06-J13A-1"
ANP_DROP_TAG="tags/v1.1.0-4D"

rocm_base_image=${3:-docker.io/rocm/dev-ubuntu-24.04:7.0}

docker build \
    --build-arg ROCM_BASE_IMAGE="${rocm_base_image}" \
    --build-arg REPO_URL="${REPO_URL}" \
    --build-arg DRIVER_LABEL="${DRIVER_LABEL}" \
    --build-arg DRIVERS_VERSION="${DRIVERS_VERSION}" \
    --build-arg IMAGE_NAME="${IMAGE_NAME}" \
    --build-arg IMAGE_VER="${IMAGE_VER}" \
    --build-arg AMD_GPU_TARGETS="${AMD_GPUS}" \
    --build-arg RCCL_DROP_TAG="${RCCL_DROP_TAG}" \
    --build-arg ANP_DROP_TAG="${ANP_DROP_TAG}" \
    -t "${IMAGE_NAME}:${IMAGE_VER}" \
    -f Dockerfile .

docker inspect ${IMAGE_NAME}:${IMAGE_VER} --format '{{ json .Config.Labels }}' | jq
