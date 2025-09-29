#!/bin/bash

# Deploy script for AMD AINIC Network Operator
# This script deploys the network operator to a Kubernetes cluster

set -e

NAMESPACE=${NAMESPACE:-network-operator-system}
IMG=${IMG:-network-operator:latest}

echo "Deploying AMD AINIC Network Operator..."
echo "Namespace: $NAMESPACE"
echo "Image: $IMG"

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "Error: kubectl is not installed or not in PATH"
    exit 1
fi

# Check if we can connect to the cluster
if ! kubectl cluster-info &> /dev/null; then
    echo "Error: Cannot connect to Kubernetes cluster"
    exit 1
fi

# Deploy the operator
echo "Installing CRDs..."
kubectl apply -f config/crd/bases/

echo "Creating namespace..."
kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

echo "Deploying RBAC and operator..."
cd config/manager && kustomize edit set image controller=$IMG
cd ../..
kustomize build config/default | kubectl apply -f -

echo "Waiting for operator to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/network-operator-controller-manager -n $NAMESPACE

echo "AMD AINIC Network Operator deployed successfully!"
echo ""
echo "To create an AINIC resource:"
echo "kubectl apply -f examples/ainic-basic.yaml"
echo ""
echo "To check the status:"
echo "kubectl get ainics"
echo "kubectl describe ainic ainic-basic"