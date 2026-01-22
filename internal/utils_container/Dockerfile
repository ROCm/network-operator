FROM registry.access.redhat.com/ubi9/ubi-minimal:9.5

LABEL name="amd-network-operator-utils"
LABEL maintainer="sriram.ravishankar@amd.com,yan.sun3@amd.com"
LABEL vendor="Advanced Micro Devices, Inc."
LABEL version="v1.1.0"
LABEL release="v1.1.0"
LABEL summary="AMD Network Operator Utility Image"
LABEL description="The AMD Network Operator utils image is a utility image used by the AMD Network Operator. The operator controller employs this image as the container's base layer to automate tasks on target worker nodes."

# Install nsenter from util-linux package
RUN microdnf install -y util-linux pciutils kmod jq && \
    cp /usr/bin/nsenter /nsenter && \
    microdnf clean all

# Define the kubectl version and ARCH
ENV KUBECTL_VERSION=v1.33.4
ENV ARCH=amd64

RUN curl -L -o /usr/local/bin/kubectl \
    https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/${ARCH}/kubectl && \
    chmod +x /usr/local/bin/kubectl

ADD LICENSE /licenses/LICENSE

# Set entrypoint to nsenter
ENTRYPOINT ["/nsenter"]
