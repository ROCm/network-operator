ARG ROCM_BASE_IMAGE
FROM $ROCM_BASE_IMAGE

ENV DEBIAN_FRONTEND=noninteractive

## Install libraries
USER root
RUN apt-get update && apt-get install -y openssh-server \
    autoconf \
    automake \
    cmake \
    git \
    xz-utils \
    pkg-config \
    ninja-build \
    libboost-dev \
    libnl-route-3-dev \
    iputils-ping \
    net-tools \
    iproute2 \
    ethtool \
    sshpass \
    libpci-dev \
    isc-dhcp-client \
    libtool \
    flex \
    unzip \
    vim

### GPUs ###
ARG AMD_GPU_TARGETS
ENV GPU_TARGETS=$AMD_GPU_TARGETS

ENV WORK_DIR=/root

### UCX install
WORKDIR $WORK_DIR
RUN git clone https://github.com/openucx/ucx.git
WORKDIR ucx
RUN git checkout v1.15.0
RUN ./autogen.sh
RUN ./contrib/configure-release --prefix=$WORK_DIR/ucx_build/ --with-rocm=/opt/rocm --disable-logging --disable-debug --disable-assertions --disable-params-check
RUN make -j16
RUN make -j16 install

### OPEN MPI install
WORKDIR $WORK_DIR
RUN git clone https://github.com/open-mpi/ompi.git
WORKDIR ompi
RUN git checkout v4.1.6
RUN ./autogen.pl
RUN ./configure --prefix=$WORK_DIR/ompi/install --with-ucx=$WORK_DIR/ucx_build/ --disable-oshmem --disable-mpi-fortran
RUN sudo make -j16
RUN sudo make -j16 install

##### RCCL  install
ARG RCCL_DROP_TAG
WORKDIR $WORK_DIR
RUN git clone https://github.com/ROCm/rccl.git
WORKDIR rccl
RUN git checkout $RCCL_DROP_TAG
RUN git status
RUN ./install.sh --amdgpu_targets=$AMD_GPU_TARGETS --prefix build/ --disable-mscclpp --disable-msccl-kernel

#### RCCL  tests install
WORKDIR $WORK_DIR
RUN git clone https://github.com/ROCmSoftwarePlatform/rccl-tests.git
WORKDIR rccl-tests/
RUN make GPU_TARGETS="$(echo $AMD_GPU_TARGETS | tr ';' ' ')" MPI=1 MPI_HOME=$WORK_DIR/ompi/install NCCL_HOME=$WORK_DIR/rccl/build/release CUSTOM_RCCL_LIB=$WORK_DIR/rccl/build/release/librccl.so

###### AINIC Repo Drivers install
ARG REPO_URL
ARG DRIVERS_VERSION
ARG DRIVER_LABEL

RUN mkdir --parents --mode=0755 /etc/apt/keyrings && \
    wget ${REPO_URL}/rocm/rocm.gpg.key -O - | \
    gpg --dearmor | tee /etc/apt/keyrings/rocm.gpg > /dev/null

RUN echo "Types: deb" > /etc/apt/sources.list.d/amdainic.sources && \
    echo "URIs: ${REPO_URL}/amdainic/pensando/ubuntu/${DRIVERS_VERSION}" >> /etc/apt/sources.list.d/amdainic.sources && \
    echo "Suites: ${DRIVER_LABEL}" >> /etc/apt/sources.list.d/amdainic.sources && \
    echo "Components: main" >> /etc/apt/sources.list.d/amdainic.sources && \
    echo "Signed-By: /etc/apt/keyrings/rocm.gpg" >> /etc/apt/sources.list.d/amdainic.sources

RUN apt-get update && apt-get install -y bc jq libibverbs-dev rdma-core ibverbs-utils
RUN apt-get update && apt-get install -y libionic-dev libionic1 perftest libfmt-dev

##### RCCL ANP install
ARG ANP_DROP_TAG
WORKDIR $WORK_DIR
RUN git clone https://github.com/ROCm/amd-anp.git
WORKDIR amd-anp
RUN git checkout $ANP_DROP_TAG
RUN git status
RUN sudo make -j RCCL_HOME=$WORK_DIR/rccl RCCL_BUILD=$WORK_DIR/rccl/build/release MPI_INCLUDE=$WORK_DIR/ompi/install/include/ MPI_LIB_PATH=$WORK_DIR/ompi/install/lib

# SSH dependencies for MPI
RUN sed -i 's/[ #]\(.*StrictHostKeyChecking \).*/ \1no/g' /etc/ssh/ssh_config && \
   echo "    UserKnownHostsFile /dev/null" >> /etc/ssh/ssh_config && \
   sed -i 's/#\(StrictModes \).*/\1no/g' /etc/ssh/sshd_config && \
   mkdir /var/run/sshd -p

RUN mkdir -p /root/.ssh && chmod 700 /root/.ssh
RUN ssh-keygen -t rsa -b 4096 -N '' -f /root/.ssh/id_rsa
RUN cat /root/.ssh/id_rsa.pub >> /root/.ssh/authorized_keys
RUN chmod 600 /root/.ssh/authorized_keys
RUN echo "PermitRootLogin yes" >> /etc/ssh/sshd_config

# User Scripts
WORKDIR /tmp
COPY run_rccl.sh /tmp/
COPY show_gid /usr/sbin/show_gid
RUN chmod +x /tmp/run_rccl.sh /usr/sbin/show_gid

# ENV setup
ENV LD_LIBRARY_PATH=$WORK_DIR/amd-anp/build:$WORK_DIR/rccl/build/release/build/lib:/opt/rocm/lib:/opt/rocm/lib64:$WORK_DIR/ompi/install/lib:$LD_LIBRARY_PATH
ENV PATH=/opt/rocm/bin:$WORK_DIR/ompi/install/bin:$PATH
ENV LD_PRELOAD=$WORK_DIR/rccl/build/release/librccl.so.1.0

ARG IMAGE_NAME
ARG IMAGE_VER
ARG REPO_URL
ARG DRIVER_LABEL
ARG DRIVERS_VERSION
ARG ROCM_BASE_IMAGE
ENV IMAGE_NAME=$IMAGE_NAME
ENV IMAGE_VER=$IMAGE_VER
ENV DRIVER_LABEL=$DRIVER_LABEL
ENV DRIVERS_VERSION=$DRIVERS_VERSION
ENV DOCKERFILE_VER="roce-workload-10dec"

LABEL image.version.name=$IMAGE_NAME
LABEL image.version.bldver=$IMAGE_VER
LABEL image.version.dkrfile=$DOCKERFILE_VER
LABEL image.libs.rccl=$RCCL_DROP_TAG
LABEL image.libs.anp=$ANP_DROP_TAG
LABEL image.libs.gpu_targets=$AMD_GPU_TARGETS
LABEL image.driver.repoURL=$REPO_URL
LABEL image.driver.label=$DRIVER_LABEL
LABEL image.driver.version=$DRIVERS_VERSION
LABEL base_image=$ROCM_BASE_IMAGE \
      name="roce-workload" \
      maintainer="sundaramurthy.gurunathan@amd.com" \
      version="v1.0.0" \
      description="Create RoCE workload docker image with ROCm, RCCL, RCCL-Tests, AMD Network Plugin, AMD AINIC drivers and libraries"

ENTRYPOINT ["sh", "-c", "service ssh restart && sleep infinity"]
