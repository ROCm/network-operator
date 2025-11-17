# Cluster Validation and Job Scheduling Framework

The **Cluster  Validation and Job Scheduling Framework** is a Kubernetes-based framework designed to periodically verify the health and readiness of worker nodes present in the cluster through active tests. This framework can also be used for validating a set of newly added worker nodes to the kubernetes cluster before scheduling distributed training,inference workloads on them.

In addition to validation, this framework can be leveraged for scheduling and orchestrating distributed AI and HPC workloads, ensuring that performance-verified nodes participate in large-scale compute jobs.

---

## Overview

The framework runs as a **Kubernetes CronJob** that:

1. Selects and labels candidate nodes based on criteria specified through a configMap.
2. Execute test cases to validate GPU performance by using ROCm Validation Suite (RVS) or AMD GPU Field Health Check (AGFHC).
3. Launches multiple distributed RCCL test OR AI/HPC workloads via MPIJob.
4. Collects and logs test results for further analysis.
5. Validates test results with performance thresholds specified through configMap.
6. Applies labels on the candidate nodes based on the test result.

This setup enables **automated, periodic validation** of cluster node health — particularly useful in GPU/compute clusters or high-performance environments.
This setup is also useful for validating new worker nodes in a k8s cluster before they are made available for GPU/AINIC workloads.

Training large models across multiple GPUs, AINICs or Nodes requires efficient inter-process communication (IPC) and resource coordination. This framework also empowers the user to run scalable, fault-tolerant, and reproducible distributed training jobs using AMD GPU and AINIC resources on a Kubernetes cluster without any manual MPI setup

---

## Requirements

The following prerequisites need to be done on each new node which is intended to be added to the k8s production cluster.

* Bringup basic k8s generic cluster with master and worker nodes and components like  kube-apiserver, controller, etcd , coredns etc
* Each worker node should have a defined number of AMD GPUs and AMD AINICs provisioned based on the deployment requirements.
* AINIC Software and drivers need to be installed on the Host and AINIC cards. AMD Network Operator can install the required kernel driver inside the VM in GA  release.
* AMD GPU drivers also need to be installed in worker node. GPU Operator can be installed to satisfy this requirement.
* AMD GPU operator and AMD Network operator need to be installed, and GPU and NIC resources reported to kubelet.
* [MPI Operator](https://github.com/kubeflow/mpi-operator/blob/master/README.md) must be installed in the cluster
* Ethernet and RoCE interfaces set up with homogeneous names / configs across the new nodes.

Back-end networking should be set up on the new node, and connectivity to other nodes in the cluster should be configured.

---

## Architecture

This framework would perform the following:

* Perform Single Node AGFHC tests on the node and apply label that indicates the node is ready for MPI-RCCL tests (To Be Added)
* Perform Single Node and Multi Node RCCL tests
* Validate the Frontend connectivity of the newly added Nodes.
* Validate the GPU, AINIC, and RDMA connectivity to the backend RoCE network.
* Validate RDMA performance results by running multiple RCCL collectives such as All Reduce, Reduce Scatter, etc.
* Compare the performance results with existing benchmarks to determine whether the node should be added to the production cluster.
* If deemed suitable, apply the appropriate Node-ready labels on the newly added node.

In addition to new node validation, the framework could also be used to schedule training jobs on an existing k8s cluster containing AMD GPUs and AINICs

This framework supports Gang Scheduling  by checking for Pod Running status and SSH connectivity

---

## Key Components

| Component | Description |
|------------|-------------|
| **CronJob** | Periodically triggers node cluster node validation checks (e.g., every 24 hours). |
| **ConfigMap** | Stores configuration, candidate selection script, Job and MPIJob manifest templates. |
| **ServiceAccount + RBAC** | Grants permission to list/label nodes and create workloads. |
| **MPIJob** | Executes RCCL collective tests across candidate nodes. |

---

## Flow Summary

1. **Candidate Node Selection**  
   * The CronJob script selects nodes with configmap driven node selectors (e.g. `feature.node.kubernetes.io/amd-nic=true`).
   * The matching nodes available after applying user specified filters are then labeled with a candidate marker (e.g. `amd.com/cluster-validation-candidate=true`).

2. **Test Runner RVS / AGFHC Execution**
   * A test runner job validates AMD GPU health and performance in parallel across all candidate nodes before executing distributed RCCL tests.

3. **RCCL Test Execution**
   * A job manifest (like `MPIJob`) is applied dynamically using `kubectl apply`.
   * The job runs distributed or node-local workloads to test network, GPU, AINIC and system health.

4. **Result Validation**
   * Test results are validated and the participating worker nodes are labelled with test status
   * The candidate marker is removed from the nodes involved in the cluster validation at the end of the CronJob.

5. **Periodic Checks**
   * This validation or job scheduling process is periodically triggered through CronJob and all available nodes in the cluster which are not part of an active workload job can be periodically qualified for performance and connectivity and used for job scheduling.

---

## 🚀 Deployment Steps

## 1. Create ConfigMaps

[Download ConfigMap YAML](../_static/cluster-validation-config.yaml)

```bash
kubectl apply -f cluster-validation-config.yaml
```

---

## 2. Deploy Cluster Validation Job

[Download CronJob YAML](../_static/cluster-validation-job.yaml)
(CronJob + MPIJob Template + RBAC)

```bash
kubectl apply -f cluster-validation-job.yaml
```

---

## 3. Verify CronJob

```bash
kubectl get cronjob cluster-validation-cron-job
kubectl get jobs
```

---

## 4. Check Node Labels

```bash
kubectl get nodes --show-labels | grep cluster-validation-status
kubectl describe node | grep "amd.com/cluster-validation\|Name:"
```

---

## 5. Inspect Logs

```bash
kubectl logs job/cluster-validation-cron-job-<29379315>
kubectl logs job/cluster-validation-mpi-job-<20251110-0715>-launcher
```

---

## Example Output Labels

| Node   | Label | Meaning |
|:--------|:--------------------------------------------|:-----------------------------------------------------------|
| node-a | `amd.com/cluster-validation-status=passed` | Node successfully passed all RCCL tests |
| node-b | `amd.com/cluster-validation-status=failed` | Node failed one or more RCCL tests |
| node-c | *(no label)* | Node not part of current candidate set |

---

## Notes for Operators

* Update image tags (**roce-workload**, **network-operator-utils**) as needed before deployment.
* `slotsPerWorker` and resource limits must match the underlying GPU/NIC configuration.  
* Modify `schedule` under `CronJob.spec` to change job frequency.  
* Use `DEBUG_DELAY` to pause after job completion for debugging failed runs.  

---

## Cleanup

To remove all cluster validation resources:

```bash
kubectl delete -f cluster-validation-job.yaml
kubectl delete -f cluster-validation-config.yaml
kubectl delete jobs --selector amd.com/cluster-validation-created=true
kubectl delete mpijobs --selector amd.com/cluster-validation-created=true
```

---
