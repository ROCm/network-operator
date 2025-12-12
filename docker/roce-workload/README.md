## RoCE Workload Docker Image Build Instructions

1. [Dockerfile](./Dockerfile) contains the docker build instructions for RoCE workload

2. Build the RoCE/RCCL workload image by running [Docker-Build.sh](./docker-build.sh) script providing the docker image tag and AINIC FW version . The docker image bundles ROCm, RCCL, RCCL tests, AMD Network Plugin and AINIC user-space drivers and packages

    ```bash
    ./docker-build.sh      tag_ver1      1.117.1-a-63
    ```

2. [Docker-Build.sh](./docker-build.sh) can be customized to provide the following args in roce-workload image creation
    * ROCM_BASE_IMAGE
    * RCCL_DROP_TAG
    * ANP_DROP_TAG
    * AINIC SW Version

3. Start the workload pod by using the roce image in [Workload.yaml](https://instinct.docs.amd.com/projects/network-operator/en/latest/installation/workload.html#deploy-the-workload)
