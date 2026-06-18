# MPI Operator Specification for RCCL Performance Testing

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Prerequisites](#prerequisites)
4. [Installation](#installation)

## Overview

To validate the performance of AMD GPU clusters for distributed AI/ML workloads, it is essential to benchmark the inter-node and intra-node communication capabilities.
The MPI Operator for Kubernetes enables the execution of Message Passing Interface (MPI) workloads on AMD GPU clusters. This specification defines how to deploy and configure the MPI Operator specifically for running RCCL (ROCm Collective Communications Library) performance tests to validate cluster performance and benchmark AMD GPU interconnect capabilities.

### Purpose

This specification provides a comprehensive guide for:

- Deploying MPI Operator in Kubernetes clusters with AMD GPUs
- Creating MPIJob resources to run RCCL performance tests
- Validating cluster performance for distributed AI/ML workloads
- Benchmarking inter-node and intra-node GPU communication

### Scope

The specification covers:

- MPI Operator installation and configuration
- Integration with AMD Network Operator and AMD GPU Operator
- Recipes for running RCCL performance tests

## Architecture

### Component Overview

```text
┌─────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                       │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐    ┌─────────────────┐                 │
│  │   MPI Operator  │    │ AMD Network     │                 │
│  │   Controller    │◄──►│ Operator        │                 │
│  └─────────────────┘    └─────────────────┘                 │
│           │                       │                         │
│           ▼                       ▼                         │
│  ┌─────────────────┐    ┌─────────────────┐                 │
│  │   MPIJob CRD    │    │ NetworkConfig   │                 │
│  └─────────────────┘    │ CRD             │                 │
│           │             └─────────────────┘                 │
│           ▼                       │                         │
│  ┌─────────────────┐              ▼                         │
│  │  Launcher Pod   │    ┌─────────────────┐                 │
│  │  - mpirun       │    │ Device Plugin   │                 │
│  │  - RCCL tests   │    │ Node Labeller   │                 │
│  └─────────────────┘    │ Metrics Export  │                 │
│           │             └─────────────────┘                 │
│           ▼                       │                         │
│  ┌─────────────────────────────────────────┐                │
│  │           Worker Pods                   │                │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  │                │
│  │  │Worker-0 │  │Worker-1 │  │Worker-N │  │                │
│  │  │AMD GPU  │  │AMD GPU  │  │AMD GPU  │  │                │
│  │  │RCCL lib │  │RCCL lib │  │RCCL lib │  │                │
│  │  └─────────┘  └─────────┘  └─────────┘  │                │
│  └─────────────────────────────────────────┘                │
└─────────────────────────────────────────────────────────────┘
```

### Key Components

1. **MPI Operator Controller**: Manages MPIJob lifecycle
2. **MPIJob Custom Resource**: Defines MPI workload specifications
3. **Launcher Pod**: Executes `mpirun` command to coordinate worker processes
4. **Worker Pods**: Run RCCL test processes on AMD GPUs
5. **AMD Network Operator**: Provides AINIC driver installation and network configuration
6. **AMD GPU Operator**: Manages GPU driver installation and management

## Prerequisites

### Cluster Requirements

- Kubernetes cluster version 1.31+
- AMD GPU nodes with ROCm drivers installed
- AMD AINIC network for high-performance interconnect
- Container runtime with GPU support (Docker/containerd with AMD GPU Operator)
- Network connectivity between nodes (InfiniBand/Ethernet for multi-node tests)

### Software Dependencies

- ROCm >= 7.0.0
- RCCL >= 2.15.0
- MPI implementation (OpenMPI, MPICH, or Intel MPI)
- AMD Network Operator (for AMD AINIC driver management)
- AMD GPU Operator (for GPU driver management)
- AINIC RCCL Container image (for RCCL test binaries)
  - Include ROCM stack, RCCL, and MPI libraries
  - AINIC NPL Plugin (for AINIC driver performance parameters)

### Hardware Requirements

- AMD GPUs (MI3XX series recommended)
- High-bandwidth interconnect (AMD AINIC preferred)
- NVMe storage for fast I/O

### Network Configuration

- Kubernetes pod network (CNI)
- High-performance secondary network for MPI communication
- Proper firewall rules for MPI port ranges
- Network time synchronization (NTP/Chrony)

## Installation

### 1. Network Operator Installation

The AMD Network Operator is required to manage the installation and configuration of the AINIC driver for high-performance interconnect.

[Install Instructions](../installation/kubernetes-helm.md#installing-the-amd-network-operator)

### 2. GPU Operator Installation

The AMD GPU Operator is required to manage the installation and configuration of the amdgpu drivers for AMD GPUs.

[Instal lInstructions](https://instinct.docs.amd.com/projects/gpu-operator/en/latest/installation/kubernetes-helm.html#)

### 3. MPI Operator Installation

TBD

## Test Recipes

TBD

### 4. Test Result Verification

TBD
