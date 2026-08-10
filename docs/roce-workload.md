# RoCE Workload Image

The RoCE workload image bundles all components needed for running distributed RCCL performance tests across AMD GPU clusters with AMD AINIC (Pollara) NICs.

## Prerequisites

- Kubernetes cluster with AMD GPU and AINIC (Pollara) NIC nodes
- [AMD Network Operator](https://instinct.docs.amd.com/projects/network-operator/en/main/) deployed with a `NetworkConfig` CR
- `amd-host-device-nad` [NetworkAttachmentDefinition](https://instinct.docs.amd.com/projects/network-operator/en/main/secondary_network/amd-host-device-cni.html) created
- [AMD GPU Operator](https://instinct.docs.amd.com/projects/gpu-operator/en/latest/) deployed (for `amd.com/gpu` device resources)
- [MPI Operator](https://github.com/kubeflow/mpi-operator) installed (for CVF and MPIJob-based workloads)

**Important:** The workload image must be compatible with the AINIC driver version installed on the host nodes. The image bundles user-space AINIC libraries (libionic) that must match the host kernel driver. Use an image tagged with the same AINIC version as your deployed drivers (e.g., `ainic-1.117.5-a-77` in the image tag should match the firmware/driver version on the nodes).

## Pre-built Images

Pre-built images are available on [Docker Hub](https://hub.docker.com/r/rocm/roce-workload).

## What's Included

| Component | Description |
| ------------- | ----------------------------------------------------- |
| ROCm | AMD GPU compute runtime |
| RCCL | ROCm Collective Communications Library |
| RCCL Tests | Performance benchmarks (all_reduce_perf, broadcast_perf, etc.) |
| AMD ANP | AMD Network Plugin for RCCL |
| UCX | Unified Communication X transport layer |
| OpenMPI | MPI implementation for multi-node communication |
| AINIC drivers | User-space libraries for AMD AINIC (Pollara) NICs |

## Quick Start

Deploy two workload pods and run an RCCL test to validate GPU-to-GPU communication over AINIC NICs:

```bash
# Deploy workload pods (adjust gpu/nic counts to match your hardware)
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
---
apiVersion: v1
kind: Pod
metadata:
  name: rccl-worker-1
  labels:
    app: rccl-test
  annotations:
    k8s.v1.cni.cncf.io/networks: amd-host-device-nad,amd-host-device-nad
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

# Exchange SSH keys between pods
W0_KEY=$(kubectl exec rccl-worker-0 -- cat /root/.ssh/id_rsa.pub)
W1_KEY=$(kubectl exec rccl-worker-1 -- cat /root/.ssh/id_rsa.pub)
kubectl exec rccl-worker-0 -- bash -c "echo '$W1_KEY' >> /root/.ssh/authorized_keys"
kubectl exec rccl-worker-1 -- bash -c "echo '$W0_KEY' >> /root/.ssh/authorized_keys"

# Get pod IPs
W0_IP=$(kubectl get pod rccl-worker-0 -o jsonpath='{.status.podIP}')
W1_IP=$(kubectl get pod rccl-worker-1 -o jsonpath='{.status.podIP}')

# Run RCCL test (set GPUS_PER_NODE to match amd.com/gpu above)
kubectl exec rccl-worker-0 -- env GPUS_PER_NODE=8 run_rccl.sh $W0_IP $W1_IP all_reduce_perf

# Cleanup
kubectl delete pod rccl-worker-0 rccl-worker-1
```

A successful run ends with:

```text
# Avg bus bandwidth    : 6.66557
```

**For VMs with VNICs (SR-IOV VFs):** replace `amd.com/nic` with `amd.com/vnic` and `amd-host-device-nad` with `vf-amd-host-device-nad`.

## Using with CVF

For fleet-wide cluster validation, use the [Cluster Validation Framework](cluster_validation_framework/README.md). Set the image in the CVF `config.yaml`:

```yaml
RCCL_WORKLOAD_IMAGE: "rocm/roce-workload:<tag>"
```

## Building a Custom Image

To build a custom image with specific component versions, see the build instructions at [`docker/roce-workload/`](https://github.com/ROCm/network-operator/tree/main/docker/roce-workload).

```bash
cd docker/roce-workload
./docker-build.sh <ainic_version>
```
