#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIRECTORY=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPOSITORY_ROOT=$(cd "$SCRIPT_DIRECTORY/../.." && pwd)

export E2E_CONTROLLER_IMAGE=${E2E_CONTROLLER_IMAGE:-cloudflare-tunnel-ingress-controller:e2e}
export E2E_MINIKUBE_PROFILE=${E2E_MINIKUBE_PROFILE:-cf-ic-e2e-${GITHUB_RUN_ID:-$$}-${GITHUB_RUN_ATTEMPT:-1}}
E2E_DUMP_DIRECTORY=${E2E_DUMP_DIRECTORY:-$REPOSITORY_ROOT/test/e2e/artifacts/cluster-dump}

REMOVE_KUBECONFIG=false
if [[ -z ${E2E_KUBECONFIG:-} ]]; then
    E2E_KUBECONFIG=$(mktemp "${TMPDIR:-/tmp}/cf-ic-e2e-kubeconfig.XXXXXX")
    REMOVE_KUBECONFIG=true
fi
export E2E_KUBECONFIG

printf 'E2E minikube profile: %s\n' "$E2E_MINIKUBE_PROFILE"

finish() {
    local status=$?
    trap - EXIT
    set +e

    export KUBECONFIG="$E2E_KUBECONFIG"
    DUMP_DIRECTORY="$E2E_DUMP_DIRECTORY" \
        bash "$SCRIPT_DIRECTORY/dump.sh"
    kubectl delete ingress dashboard-via-cloudflare \
        --namespace kubernetes-dashboard \
        --ignore-not-found=true \
        --wait=true
    kubectl delete ingress redis-via-cloudflare-tcp \
        --namespace default \
        --ignore-not-found=true \
        --wait=true
    kubectl delete ingress wildcard-routing-via-cloudflare \
        --namespace default \
        --ignore-not-found=true \
        --wait=true
    minikube -p "$E2E_MINIKUBE_PROFILE" addons disable dashboard
    minikube -p "$E2E_MINIKUBE_PROFILE" addons disable metrics-server
    helm uninstall cf-ic-e2e \
        --namespace cloudflare-tunnel-ingress-controller \
        --wait
    minikube delete -p "$E2E_MINIKUBE_PROFILE"

    if [[ $REMOVE_KUBECONFIG == true ]]; then
        rm -f "$E2E_KUBECONFIG"
    fi

    exit "$status"
}

trap finish EXIT

cd "$REPOSITORY_ROOT"
go test -timeout 30m -v ./test/e2e
