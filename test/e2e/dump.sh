#!/usr/bin/env bash

set -u
set -o pipefail

DUMP_DIRECTORY=${DUMP_DIRECTORY:-test/e2e/artifacts/cluster-dump}
MANIFEST_DIRECTORY="$DUMP_DIRECTORY/manifests"
LOG_DIRECTORY="$DUMP_DIRECTORY/logs"

mkdir -p "$MANIFEST_DIRECTORY" "$LOG_DIRECTORY"

capture() {
    local output=$1
    shift
    "$@" > "$output" 2>&1 || true
}

kubectl cluster-info dump \
    --all-namespaces \
    --output-directory "$DUMP_DIRECTORY/cluster-info" \
    -o yaml || true

capture "$MANIFEST_DIRECTORY/resources.txt" kubectl get all -A -o wide
capture "$MANIFEST_DIRECTORY/events.yaml" kubectl get events -A --sort-by=.metadata.creationTimestamp -o yaml
capture "$MANIFEST_DIRECTORY/endpoints.yaml" kubectl get endpoints -A -o yaml
capture "$MANIFEST_DIRECTORY/endpoint-slices.yaml" kubectl get endpointslices.discovery.k8s.io -A -o yaml
capture "$MANIFEST_DIRECTORY/ingresses.yaml" kubectl get ingresses.networking.k8s.io -A -o yaml
capture "$MANIFEST_DIRECTORY/ingress-classes.yaml" kubectl get ingressclasses.networking.k8s.io -o yaml
capture "$MANIFEST_DIRECTORY/configmaps.yaml" kubectl get configmaps -A -o yaml
capture "$MANIFEST_DIRECTORY/pods-describe.txt" kubectl describe pods -A
capture "$MANIFEST_DIRECTORY/deployments-describe.txt" kubectl describe deployments -A
capture "$MANIFEST_DIRECTORY/ingresses-describe.txt" kubectl describe ingresses.networking.k8s.io -A

kubectl get secrets -A -o json 2> "$MANIFEST_DIRECTORY/secrets.stderr" \
    | jq 'del(.items[].data, .items[].stringData, .items[].metadata.annotations, .items[].metadata.managedFields)' \
        > "$MANIFEST_DIRECTORY/secrets.json" || true

kubectl logs \
    -n cloudflare-tunnel-ingress-controller \
    -l app.kubernetes.io/instance=cf-ic-e2e \
    --all-containers=true \
    --prefix=true \
    --tail=-1 2>&1 \
    | tee "$LOG_DIRECTORY/controller.log" || true

kubectl logs \
    -n cloudflare-tunnel-ingress-controller \
    -l app.kubernetes.io/instance=cf-ic-e2e \
    --all-containers=true \
    --prefix=true \
    --previous \
    --tail=-1 2>&1 \
    | tee "$LOG_DIRECTORY/controller-previous.log" || true

kubectl logs \
    -n cloudflare-tunnel-ingress-controller \
    -l strrl.dev/cloudflare-tunnel-ingress-controller=controlled-cloudflared-connector \
    --all-containers=true \
    --prefix=true \
    --tail=-1 2>&1 \
    | tee "$LOG_DIRECTORY/cloudflared.log" || true

kubectl logs \
    -n cloudflare-tunnel-ingress-controller \
    -l strrl.dev/cloudflare-tunnel-ingress-controller=controlled-cloudflared-connector \
    --all-containers=true \
    --prefix=true \
    --previous \
    --tail=-1 2>&1 \
    | tee "$LOG_DIRECTORY/cloudflared-previous.log" || true
