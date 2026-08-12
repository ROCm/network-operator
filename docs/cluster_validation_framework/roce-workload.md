# RoCE Workload Image

The RoCE workload image bundles all components needed for running distributed RCCL performance tests across AMD GPU clusters with AMD AINIC (Pollara) NICs.

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

## Getting the Image

### Pre-built Images

Pre-built images are available on [Docker Hub](https://hub.docker.com/r/rocm/roce-workload).

**Important:** The workload image must be compatible with the AINIC driver version installed on the host nodes. The image bundles user-space AINIC libraries (libionic) that must match the host kernel driver. Use an image tagged with the same AINIC version as your deployed drivers (e.g., `ainic-1.117.5-a-77` in the image tag should match the firmware/driver version on the nodes).

### Building a Custom Image

To build a custom image with specific component versions, see the build instructions at [`docker/roce-workload/`](https://github.com/ROCm/network-operator/tree/main/docker/roce-workload).

```bash
cd docker/roce-workload
./docker-build.sh <ainic_version>
```

## Using the Image

The image can be used in two ways: **manually** for ad-hoc testing, or **with CVF** for automated fleet-wide validation.

Both workflows require the cluster prerequisites described in the [Cluster Validation Framework requirements](README.md#requirements).

### Manual Quick Start

Deploy two workload pods and run an RCCL test to validate GPU-to-GPU communication over AINIC NICs:

#### 1. Deploy workload pods

```bash
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
        amd.com/gpu: 8   # adjust to match your hardware
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
```

Adjust `amd.com/gpu` and `amd.com/nic` counts to match your hardware. The anti-affinity rule ensures pods land on different nodes automatically.

**For VMs with VNICs (SR-IOV VFs):** replace `amd.com/nic` with `amd.com/vnic` and `amd-host-device-nad` with `vf-amd-host-device-nad` in the pod spec above.

#### 2. Exchange SSH keys

```bash
W0_KEY=$(kubectl exec rccl-worker-0 -- cat /root/.ssh/id_rsa.pub)
W1_KEY=$(kubectl exec rccl-worker-1 -- cat /root/.ssh/id_rsa.pub)
kubectl exec rccl-worker-0 -- bash -c "echo '$W1_KEY' >> /root/.ssh/authorized_keys"
kubectl exec rccl-worker-1 -- bash -c "echo '$W0_KEY' >> /root/.ssh/authorized_keys"
```

#### 3. Run RCCL test

```bash
# Get pod IPs
W0_IP=$(kubectl get pod rccl-worker-0 -o jsonpath='{.status.podIP}')
W1_IP=$(kubectl get pod rccl-worker-1 -o jsonpath='{.status.podIP}')

# Run (GPUS_PER_NODE must match amd.com/gpu in the pod spec above)
kubectl exec rccl-worker-0 -- env GPUS_PER_NODE=8 run_rccl.sh $W0_IP $W1_IP all_reduce_perf
```

Available collectives: `all_reduce_perf`, `broadcast_perf`, `reduce_scatter_perf`, `all_gather_perf`, `alltoall_perf`, `reduce_perf`, `scatter_perf`, `gather_perf`, `sendrecv_perf`

A successful run ends with:

```text
# Avg bus bandwidth    : 6.66557
```

#### 4. Cleanup

```bash
kubectl delete pod rccl-worker-0 rccl-worker-1
```

### Automated Validation with CVF

For automated, fleet-wide cluster validation, use the [Cluster Validation Framework](README.md) instead of deploying pods manually. Set the image in the CVF `config.yaml`:

```yaml
RCCL_WORKLOAD_IMAGE: "rocm/roce-workload:<tag>"
```

See the [CVF deployment steps](README.md#deployment-steps) for the full setup.
