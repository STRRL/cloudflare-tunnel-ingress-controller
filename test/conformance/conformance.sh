#!/usr/bin/env bash
# Gateway API conformance through the real Cloudflare edge.
#
#   conformance.sh up       clean leftovers, start minikube, install CRDs, deploy
#   conformance.sh deploy   build the image and helm upgrade the controller
#   conformance.sh run      run the suite, RUN_TEST=<name> runs one test
#   conformance.sh down     remove test namespaces, the release, Cloudflare leftovers and the cluster
#   conformance.sh clean    only remove Cloudflare leftovers, works without a cluster
#
# Credentials come from ENV_FILE (default ./.env.e2e) and are never printed.

set -euo pipefail

SCRIPT_DIRECTORY=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPOSITORY_ROOT=$(cd "$SCRIPT_DIRECTORY/../.." && pwd)
cd "$REPOSITORY_ROOT"

ENV_FILE=${ENV_FILE:-./.env.e2e}
if [[ ! -f $ENV_FILE ]]; then
    echo "env file $ENV_FILE not found" >&2
    exit 1
fi
set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

GATEWAY_API_VERSION=${GATEWAY_API_VERSION:-v1.6.2}
RUN_ID=${RUN_ID:-local-$(whoami)}
export RUN_ID
PROFILE=ctic-gwc-$RUN_ID
RELEASE=ctic
RELEASE_NAMESPACE=ctic
ARTIFACTS=$SCRIPT_DIRECTORY/artifacts
mkdir -p "$ARTIFACTS"
export KUBECONFIG=$ARTIFACTS/kubeconfig-$RUN_ID

TIMEOUTS='MaxTimeToConsistency:90;GatewayMustHaveAddress:240;GatewayMustHaveCondition:240'

clean() {
    go run ./test/conformance/cleanup
}

deploy() {
    local tag
    tag=dev-$(date +%s)
    DOCKER_BUILDKIT=1 docker build -t "cloudflare-tunnel-ingress-controller:$tag" -f image/cloudflare-tunnel-ingress-controller/Dockerfile .
    minikube -p "$PROFILE" image load "cloudflare-tunnel-ingress-controller:$tag"
    helm upgrade --install "$RELEASE" ./helm/cloudflare-tunnel-ingress-controller \
        -n "$RELEASE_NAMESPACE" --create-namespace --wait \
        --set-string image.repository=cloudflare-tunnel-ingress-controller \
        --set-string image.tag="$tag" \
        --set-string image.pullPolicy=Never \
        --set-string cloudflare.accountId="$CLOUDFLARE_ACCOUNT_ID" \
        --set-string cloudflare.apiToken="$CLOUDFLARE_API_TOKEN" \
        --set-string cloudflare.tunnelName="$CLOUDFLARE_TUNNEL_NAME-gwc-$RUN_ID" \
        --set gatewayAPI.enabled=true \
        --set-string gatewayAPI.baseDomain="$E2E_BASE_DOMAIN" \
        --set-string gatewayAPI.labelSuffix="-gwc-$RUN_ID" \
        --set leaderElection.enabled=false \
        --set-string resources.limits.memory=512Mi \
        --set-string resources.requests.memory=128Mi \
        --set-string resources.limits.cpu=1 \
        >/dev/null
    echo "deployed image tag $tag"
}

up() {
    clean
    minikube start -p "$PROFILE" --driver=docker --wait=all --cpus="${MINIKUBE_CPUS:-4}" --memory="${MINIKUBE_MEMORY:-8g}"
    kubectl apply --server-side -f "https://github.com/kubernetes-sigs/gateway-api/releases/download/$GATEWAY_API_VERSION/standard-install.yaml"
    kubectl wait --for=condition=Established crd --all --timeout=2m
    deploy
}

run() {
    local args=(
        --gateway-class=cloudflare-tunnel
        "--timeout-config-overrides=$TIMEOUTS"
    )
    if [[ -n ${RUN_TEST:-} ]]; then
        args+=(--run-test="$RUN_TEST" --cleanup-base-resources=false --debug)
    else
        args+=(--cleanup-base-resources=true "--report-output=$ARTIFACTS/report.yaml")
    fi
    if [[ -n ${CONFORMANCE_ARGS:-} ]]; then
        # shellcheck disable=SC2206
        args+=($CONFORMANCE_ARGS)
    fi
    go test ./test/conformance -run TestConformance -count=1 -v -timeout 90m -args "${args[@]}"
}

down() {
    set +e
    kubectl delete ns gateway-conformance-infra gateway-conformance-app-backend gateway-conformance-web-backend --wait --timeout=3m --ignore-not-found
    # give the controller time to remove the DNS records of deleted Gateways
    sleep 15
    helm uninstall "$RELEASE" -n "$RELEASE_NAMESPACE" --wait
    set -e
    clean
    minikube delete -p "$PROFILE"
    rm -f "$KUBECONFIG"
}

case ${1:-} in
up) up ;;
deploy) deploy ;;
run) run ;;
down) down ;;
clean) clean ;;
*)
    echo "usage: $0 up|deploy|run|down|clean" >&2
    exit 1
    ;;
esac
