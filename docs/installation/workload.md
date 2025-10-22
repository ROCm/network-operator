## Deploy a Workload with a Network Device

### 1. Create a NetworkAttachmentDefinition 

Create a Network Attachment Definition to assign requested device to a workload
```bash
cat <<EOF > amd-host-device-nad.yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: amd-host-device-nad
  annotations:
    k8s.v1.cni.cncf.io/resourceName: amd.com/vnic
spec:
  config: '{
  "name": "amd-host-device-nad",
  "cniVersion": "0.3.1",
  "type": "amd-host-device"
}'
EOF
```
This step defines a secondary network using a NetworkAttachmentDefinition, which ensures the requested NIC or vNIC device is assigned to the workload pod via Multus.

### 2. Deploy the Workload

Create a workload requesting for a nic/vnic
```bash
cat <<EOF > workload.yaml
apiVersion: apps/v1 
kind: Deployment 
metadata: 
  name: workload-app 
spec: 
  replicas: 2
  selector: 
    matchLabels: 
      app: workload-app 
  template: 
    metadata: 
      annotations: 
        k8s.v1.cni.cncf.io/networks: amd-host-device-nad
      labels: 
        app: workload-app
    spec:
        hostNetwork: false
        containers:
          - name: workload-container
            image: docker.io/rocm/roce-workload:ubuntu24_rocm7_rccl-J13A-1_anp-v1.1.0-4D_ainic-1.117.1-a-63
            imagePullPolicy: IfNotPresent
            workingDir: /tmp
            command: ["/bin/bash", "-c"]
            args:
              - |
                /tmp/container_setup.sh
            securityContext:
              capabilities:
                add: 
                  - IPC_LOCK
                  - NET_ADMIN
            resources:
              requests:
                amd.com/gpu: 1
                amd.com/nic: 1
              limits:
                amd.com/gpu: 1
                amd.com/nic: 1
```

### 3. Run IB and RCCL Tests

#### 3.1 Run IB between the nodes
Exec into the workload pods and run IB and RCCL tests between the nodes. 

On node1, start the write bandwidth test using the local RoCE device:

```bash
root@app:/tmp# ib_write_bw -d roce_ai1 -i 1 -n 1000 -F -a -x 1 -q 1
```

On node2, run the write bandwidth test targeting node1's IP address, specifying its local RoCE device:
```bash
root@app:/tmp# ib_write_bw -d ionic_0 -i 1 -n 1000 -F -a -x 1 -q 1  55.1.1.56
```

Note:
`roce_ai1` and `ionic_1` are the RoCE devices available on the respective pods.
You can list available RDMA devices by running `ibv_devices` inside the pod or workload container.

#### 3.2 Run RCCL between the nodes

```bash
root@app:/tmp# /tmp/vf_rccl_run.sh
```
