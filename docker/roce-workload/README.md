## RoCE Workload Docker Image Build Instructions

1. [Dockerfile](./Dockerfile) contains the docker build instructions for RoCE workload

2. Build the RoCE/RCCL workload image by running [Docker-Build.sh](./docker-build.sh) script providing the docker image tag and AINIC FW version . The docker image bundles ROCm, RCCL, RCCL tests, AMD Network Plugin and AINIC user-space drivers and packages

    ```bash
    ./docker-build.sh      tag_ver1      1.117.1-a-63
    ```

3. [Docker-Build.sh](./docker-build.sh) can be customized to provide the following args in roce-workload image creation
    * ROCM_BASE_IMAGE
    * RCCL_DROP_TAG
    * ANP_DROP_TAG
    * AINIC SW Version

4. Start the workload pods by specifying the roce image in [Workload.yaml](https://instinct.docs.amd.com/projects/network-operator/en/latest/installation/workload.html#deploy-the-workload)

5. In [Workload.yaml](https://instinct.docs.amd.com/projects/network-operator/en/latest/installation/workload.html#deploy-the-workload), if `command:` is needed to be used when starting the container, do not forget to include SSH service restart. For ex: 

    ```bash
    command: ["sh", "-c", "service ssh restart && sleep infinity"]
    ```

6. Exec into one of the workload pods and start dual-node RCCL run using [run-rccl.sh](run_rccl.sh) packaged as part of the workload docker image

    ```bash
    /tmp/run_rccl.sh <workload-pod1-ip> <workload-pod2-ip>

    Ex: root@workload:/tmp# /tmp/run_rccl.sh 10.244.0.141 10.244.1.44
    ```
