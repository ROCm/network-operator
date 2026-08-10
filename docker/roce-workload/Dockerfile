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
    libpci-dev \
    isc-dhcp-client \
    libtool \
    flex \
    unzip \
    vim

### GPUs ###
ARG GPU_TARGETS
ENV GPU_TARGETS=$GPU_TARGETS

ENV WORK_DIR=/root

### UCX install
ARG UCX_VERSION=v1.18.0
WORKDIR $WORK_DIR
RUN git clone https://github.com/openucx/ucx.git
WORKDIR ucx
RUN git checkout $UCX_VERSION
RUN ./autogen.sh
RUN ./contrib/configure-release --prefix=$WORK_DIR/ucx_build/ --with-rocm=/opt/rocm --disable-logging --disable-debug --disable-assertions --disable-params-check
RUN make -j$(nproc)
RUN make -j$(nproc) install

### OPEN MPI install
ARG OMPI_VERSION=v4.1.6
WORKDIR $WORK_DIR
RUN git clone https://github.com/open-mpi/ompi.git
WORKDIR ompi
RUN git checkout $OMPI_VERSION
RUN ./autogen.pl
RUN ./configure --prefix=$WORK_DIR/ompi/install --with-ucx=$WORK_DIR/ucx_build/ --disable-oshmem --disable-mpi-fortran
RUN make -j$(nproc)
RUN make -j$(nproc) install

##### RCCL install
ARG RCCL_TAG
WORKDIR $WORK_DIR
RUN git clone https://github.com/ROCm/rccl.git
WORKDIR rccl
RUN git checkout $RCCL_TAG
RUN ./install.sh --amdgpu_targets=$GPU_TARGETS --prefix build/ --disable-msccl-kernel

#### RCCL tests install
ARG RCCL_TESTS_SHA
WORKDIR $WORK_DIR
RUN git clone --filter=blob:none --sparse --no-checkout \
      https://github.com/ROCm/rocm-systems.git rocm-systems && \
    cd rocm-systems && \
    git sparse-checkout set projects/rccl-tests && \
    git fetch --depth 1 origin ${RCCL_TESTS_SHA} && \
    git checkout ${RCCL_TESTS_SHA}
WORKDIR $WORK_DIR/rocm-systems/projects/rccl-tests
RUN make GPU_TARGETS="$(echo $GPU_TARGETS | tr ';' ' ')" MPI=1 MPI_HOME=$WORK_DIR/ompi/install NCCL_HOME=$WORK_DIR/rccl/build/release CUSTOM_RCCL_LIB=$WORK_DIR/rccl/build/release/librccl.so
RUN ln -sf $WORK_DIR/rocm-systems/projects/rccl-tests $WORK_DIR/rccl-tests

###### AINIC drivers install (via apt repo)
ARG REPO_URL
ARG AINIC_VERSION

RUN mkdir --parents --mode=0755 /etc/apt/keyrings && \
    wget ${REPO_URL}/rocm/rocm.gpg.key -O - | \
    gpg --dearmor | tee /etc/apt/keyrings/rocm.gpg > /dev/null

RUN DRIVER_LABEL=$(. /etc/os-release && echo "$VERSION_CODENAME") && \
    echo "Types: deb" > /etc/apt/sources.list.d/amdainic.sources && \
    echo "URIs: ${REPO_URL}/amdainic/pensando/ubuntu/${AINIC_VERSION}" >> /etc/apt/sources.list.d/amdainic.sources && \
    echo "Suites: ${DRIVER_LABEL}" >> /etc/apt/sources.list.d/amdainic.sources && \
    echo "Components: main" >> /etc/apt/sources.list.d/amdainic.sources && \
    echo "Signed-By: /etc/apt/keyrings/rocm.gpg" >> /etc/apt/sources.list.d/amdainic.sources

RUN rm -rf /var/lib/apt/lists/* && \
    apt-get update && \
    apt-get install -y bc jq libibverbs-dev rdma-core ibverbs-utils \
    libionic-dev libionic1 perftest libfmt-dev

##### RCCL ANP install
ARG ANP_TAG
WORKDIR $WORK_DIR
RUN git clone https://github.com/ROCm/amd-anp.git amd-anp
WORKDIR $WORK_DIR/amd-anp
RUN git checkout $ANP_TAG
RUN make -j RCCL_HOME=$WORK_DIR/rccl MPI_INCLUDE=$WORK_DIR/ompi/install/include/ MPI_LIB_PATH=$WORK_DIR/ompi/install/lib

# SSH setup
RUN sed -i 's/[ #]\(.*StrictHostKeyChecking \).*/ \1no/g' /etc/ssh/ssh_config && \
   echo "    UserKnownHostsFile /dev/null" >> /etc/ssh/ssh_config && \
   sed -i 's/#\(StrictModes \).*/\1no/g' /etc/ssh/sshd_config && \
   mkdir /var/run/sshd -p

RUN mkdir -p /root/.ssh && chmod 700 /root/.ssh
RUN echo "PermitRootLogin yes" >> /etc/ssh/sshd_config

# User scripts
WORKDIR /tmp
COPY show_gid /usr/sbin/show_gid
COPY run_rccl.sh /usr/local/bin/
COPY entrypoint.sh /root/entrypoint.sh
RUN chmod +x /usr/local/bin/run_rccl.sh /usr/sbin/show_gid /root/entrypoint.sh

# ENV setup
ENV LD_LIBRARY_PATH=$WORK_DIR/amd-anp/build:$WORK_DIR/rccl/build/release/build/lib:/opt/rocm/lib:/opt/rocm/lib64:$WORK_DIR/ompi/install/lib:$LD_LIBRARY_PATH
ENV PATH=/opt/rocm/bin:$WORK_DIR/ompi/install/bin:$PATH
ENV LD_PRELOAD=$WORK_DIR/rccl/build/release/librccl.so.1.0

ARG IMAGE_NAME
ARG IMAGE_TAG
ARG ROCM_BASE_IMAGE
ENV IMAGE_NAME=$IMAGE_NAME
ENV IMAGE_TAG=$IMAGE_TAG
ENV AINIC_VERSION=$AINIC_VERSION

LABEL image.version.name=$IMAGE_NAME
LABEL image.version.tag=$IMAGE_TAG
LABEL image.libs.rccl=$RCCL_TAG
LABEL image.libs.rccl_tests=$RCCL_TESTS_SHA
LABEL image.libs.anp=$ANP_TAG
LABEL image.libs.gpu_targets=$GPU_TARGETS
LABEL image.driver.repoURL=$REPO_URL
LABEL image.driver.version=$AINIC_VERSION
LABEL base_image=$ROCM_BASE_IMAGE \
      name="roce-workload" \
      maintainer="sundaramurthy.gurunathan@amd.com" \
      version="v1.0.3" \
      description="RoCE workload image with ROCm, RCCL, RCCL-Tests, AMD ANP, AINIC drivers"

ENTRYPOINT ["/root/entrypoint.sh"]
