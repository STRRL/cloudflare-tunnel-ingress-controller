#!/bin/bash

set -euo pipefail

echo "=== E2E Environment Cleanup ==="

# Remove Helm deployment
echo "Removing controller Helm deployment..."
if helm list --namespace=cloudflare-tunnel-ingress-controller | grep -q cloudflare-tunnel-ingress-controller; then
    helm uninstall cloudflare-tunnel-ingress-controller --namespace=cloudflare-tunnel-ingress-controller
    echo "Controller deployment removed"
else
    echo "Controller deployment not found, skipping"
fi

# Remove namespace (this also removes the secret)
echo "Removing controller namespace..."
if kubectl get namespace cloudflare-tunnel-ingress-controller >/dev/null 2>&1; then
    kubectl delete namespace cloudflare-tunnel-ingress-controller --timeout=60s
    echo "Controller namespace removed"
else
    echo "Controller namespace not found, skipping"
fi

# Clean up any test namespaces (e2e-test-*)
echo "Cleaning up test namespaces..."
TEST_NAMESPACES=$(kubectl get namespaces -o name | grep "namespace/e2e-test-" || true)
if [ -n "$TEST_NAMESPACES" ]; then
    echo "$TEST_NAMESPACES" | xargs kubectl delete --timeout=60s
    echo "Test namespaces cleaned up"
else
    echo "No test namespaces found"
fi

# Remove image from minikube
echo "Removing image from minikube..."
if minikube status >/dev/null 2>&1; then
    minikube image rm ghcr.io/strrl/cloudflare-tunnel-ingress-controller:dev || true
    echo "Image removed from minikube"
else
    echo "Minikube not running, skipping image removal"
fi

echo "=== E2E Environment Cleanup Complete ==="
echo "Note: minikube cluster is still running. Stop it with 'minikube stop' if desired."