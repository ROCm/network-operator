# E2E Tests

This document describes how to run end-to-end (E2E) tests against an existing, running Kubernetes cluster to verify that all tests are functioning correctly.

## Prerequisites

Ensure the following tools are installed on the system where the tests will be executed:

- Go 1.22 or later
- Helm v3.2.0 or later
- kubectl

## Running E2E Tests Against a Running Kubernetes Cluster

1. **Configure kubeconfig**

   Copy the kubeconfig file from the target Kubernetes cluster to the local machine where the tests will be run.
   By default, the kubeconfig should be located at:

   ```bash
   $HOME/.kube/config
   ```

   If you are using a kubeconfig file in a different location, provide the absolute path using `TEST_ARGS`. For example:

   ```bash
   TEST_ARGS=--kubeConfig=/tmp/kube/config
   ```

2. **Verify cluster access**

   Ensure that `kubectl` is correctly configured and pointing to the target cluster. Running standard `kubectl` commands should list resources from that cluster.

3. **Test directories and structure**

   Both the `e2e/` and `helm-e2e/` directories contain a `Makefile`.
   Running `make` in either directory will execute all tests defined within that directory.

   - `helm-e2e/`
     Contains Helm-related tests that validate installation and uninstallation of the network operator.

   - `e2e/`
     Contains tests focused on network operator functionality, including:

     - Pod and operand health checks
     - Operand upgrade and downgrade scenarios
     - Exporter validation
     - Cluster-level tests

4. **Run the tests**

   Navigate to the desired test directory (`e2e/` or `helm-e2e/`) and run:

   ```bash
   make
   ```

   This will execute all tests for that test suite.
