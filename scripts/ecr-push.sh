#!/usr/bin/env bash
# Build the controller image for linux/amd64 and push to FGT ECR.
# Usage: ./scripts/ecr-push.sh [tag]
#   tag  Optional image tag. Defaults to githash-<short-sha>.
#
# Prerequisites:
#   - AWS CLI configured (SSO or env vars)
#   - Docker with buildx installed
#   - aws-actions/amazon-ecr-login equivalent: handled below via aws ecr get-login-password

set -euo pipefail

AWS_ACCOUNT_ID="${FGT_PRIMARY_AWS_ACCOUNT_ID:?FGT_PRIMARY_AWS_ACCOUNT_ID is not set}"
AWS_REGION="${AWS_REGION:-us-east-1}"
ECR_REPOSITORY="cloudflare-tunnel-ingress-controller"
REGISTRY="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"
REPO_URI="${REGISTRY}/${ECR_REPOSITORY}"

SHA=$(git rev-parse --short HEAD)
TAG="${1:-githash-${SHA}}"
IMAGE="${REPO_URI}:${TAG}"

echo "==> Checking ECR repository '${ECR_REPOSITORY}'..."
if ! aws ecr describe-repositories \
    --repository-names "${ECR_REPOSITORY}" \
    --region "${AWS_REGION}" > /dev/null 2>&1; then
  echo "==> Repository not found, creating..."
  aws ecr create-repository \
    --repository-name "${ECR_REPOSITORY}" \
    --region "${AWS_REGION}" \
    --image-scanning-configuration scanOnPush=true \
    --image-tag-mutability MUTABLE
  echo "==> Created ${ECR_REPOSITORY}"
else
  echo "==> Repository exists"
fi

echo "==> Logging in to ECR..."
aws ecr get-login-password --region "${AWS_REGION}" \
  | docker login --username AWS --password-stdin "${REGISTRY}"

echo "==> Building linux/amd64 image..."
docker buildx build \
  --platform linux/amd64 \
  --file image/cloudflare-tunnel-ingress-controller/Dockerfile \
  --tag "${IMAGE}" \
  --push \
  .

echo ""
echo "==> Pushed: ${IMAGE}"
echo ""
echo "Update infrastructure HelmRelease values:"
echo "  image:"
echo "    repository: ${REPO_URI}"
echo "    tag: ${TAG}"
