ARG BASE_IMAGE=registry.access.redhat.com/ubi9/ubi-minimal:9.3

FROM ${BASE_IMAGE}

WORKDIR /

ARG VERSION=v1.7.1
ARG TARGETARCH
ENV TARGETARCH=${TARGETARCH:-amd64}

RUN microdnf -y update && \
    microdnf -y install tar gzip && \
    microdnf clean all

RUN mkdir -p /usr/src/cni/bin && \
    if [ -n "$CNI_PLUGINS_LOCAL_PATH" ]; then \
      echo "Using local CNI plugins asset: $CNI_PLUGINS_LOCAL_PATH"; \
      tar -xvf "$CNI_PLUGINS_LOCAL_PATH" -C /usr/src/cni/bin/; \
    else \
      echo "Downloading CNI plugins from: https://github.com/containernetworking/plugins/releases/download/${VERSION}/cni-plugins-linux-${TARGETARCH}-${VERSION}.tgz"; \
      curl -L -O https://github.com/containernetworking/plugins/releases/download/${VERSION}/cni-plugins-linux-${TARGETARCH}-${VERSION}.tgz && \
      tar -xvf cni-plugins-linux-${TARGETARCH}-${VERSION}.tgz -C /usr/src/cni/bin/; \
    fi && \
    echo done

LABEL io.k8s.display-name="Container Network Plugins"

COPY ./amd-host-device /usr/src/cni/bin
ADD ./entrypoint.sh /
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]