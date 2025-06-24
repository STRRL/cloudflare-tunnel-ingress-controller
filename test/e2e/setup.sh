#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

echo "=== E2E Environment Setup ==="

# Load environment variables
if [ -f "${SCRIPT_DIR}/.env" ]; then
    echo "Loading configuration from .env file..."
    set -a
    source "${SCRIPT_DIR}/.env"
    set +a
else
    echo "Error: .env file not found. Please copy .env.example to .env and configure it."
    exit 1
fi

# Validate required environment variables
if [ -z "${CLOUDFLARE_API_TOKEN:-}" ]; then
    echo "Error: CLOUDFLARE_API_TOKEN is required"
    exit 1
fi

if [ -z "${CLOUDFLARE_ACCOUNT_ID:-}" ]; then
    echo "Error: CLOUDFLARE_ACCOUNT_ID is required"
    exit 1
fi

if [ -z "${CLOUDFLARE_TUNNEL_NAME:-}" ]; then
    echo "Error: CLOUDFLARE_TUNNEL_NAME is required"
    exit 1
fi

if [ -z "${CLOUDFLARE_TEST_DOMAIN_SUFFIX:-}" ]; then
    echo "Error: CLOUDFLARE_TEST_DOMAIN_SUFFIX is required"
    exit 1
fi

# Check if minikube is running
if ! minikube status >/dev/null 2>&1; then
    echo "Starting minikube..."
    minikube start
else
    echo "Minikube is already running"
fi

# Build controller image with dev tag
echo "Building controller image..."
cd "${PROJECT_ROOT}"
DOCKER_BUILDKIT=1 docker build -t ghcr.io/strrl/cloudflare-tunnel-ingress-controller:dev \
    -f ./image/cloudflare-tunnel-ingress-controller/Dockerfile .

# Load image into minikube
echo "Loading image into minikube..."
minikube image load ghcr.io/strrl/cloudflare-tunnel-ingress-controller:dev

# Create namespace for controller
echo "Creating controller namespace..."
kubectl create namespace cloudflare-tunnel-ingress-controller --dry-run=client -o yaml | kubectl apply -f -

# Create secret with Cloudflare credentials
echo "Creating Cloudflare credentials secret..."
kubectl create secret generic cloudflare-credentials \
    --namespace=cloudflare-tunnel-ingress-controller \
    --from-literal=api-token="${CLOUDFLARE_API_TOKEN}" \
    --from-literal=account-id="${CLOUDFLARE_ACCOUNT_ID}" \
    --dry-run=client -o yaml | kubectl apply -f -

# Install/upgrade controller with Helm
echo "Installing/upgrading controller with Helm..."
helm upgrade --install cloudflare-tunnel-ingress-controller \
    "${PROJECT_ROOT}/helm/cloudflare-tunnel-ingress-controller" \
    --namespace=cloudflare-tunnel-ingress-controller \
    --set image.repository=ghcr.io/strrl/cloudflare-tunnel-ingress-controller \
    --set image.tag=dev \
    --set image.pullPolicy=Never \
    --set cloudflare.apiToken="${CLOUDFLARE_API_TOKEN}" \
    --set cloudflare.accountId="${CLOUDFLARE_ACCOUNT_ID}" \
    --set cloudflare.tunnelName="${CLOUDFLARE_TUNNEL_NAME}" \
    --wait --timeout=300s

# Wait for controller to be ready
echo "Waiting for controller to be ready..."
kubectl wait --for=condition=available deployment/cloudflare-tunnel-ingress-controller \
    --namespace=cloudflare-tunnel-ingress-controller --timeout=300s

echo "=== E2E Environment Setup Complete ==="
echo "Controller is ready. You can now run E2E tests with 'make e2e-run'"