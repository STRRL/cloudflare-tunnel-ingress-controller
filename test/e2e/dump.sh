#!/usr/bin/env bash

set -u
set -o pipefail

DUMP_DIRECTORY=${DUMP_DIRECTORY:-test/e2e/artifacts/cluster-dump}
MANIFEST_DIRECTORY="$DUMP_DIRECTORY/manifests"
LOG_DIRECTORY="$DUMP_DIRECTORY/logs"

mkdir -p "$MANIFEST_DIRECTORY" "$LOG_DIRECTORY"

kubectl cluster-info dump \
    --all-namespaces \
    --output-directory "$DUMP_DIRECTORY/cluster-info" \
    -o yaml || true

kubectl get all -A -o wide > "$MANIFEST_DIRECTORY/resources.txt" 2>&1 || true
kubectl get events -A --sort-by=.metadata.creationTimestamp -o yaml > "$MANIFEST_DIRECTORY/events.yaml" 2>&1 || true
kubectl get endpoints -A -o yaml > "$MANIFEST_DIRECTORY/endpoints.yaml" 2>&1 || true
kubectl get endpointslices.discovery.k8s.io -A -o yaml > "$MANIFEST_DIRECTORY/endpoint-slices.yaml" 2>&1 || true
kubectl get ingresses.networking.k8s.io -A -o yaml > "$MANIFEST_DIRECTORY/ingresses.yaml" 2>&1 || true
kubectl get ingressclasses.networking.k8s.io -o yaml > "$MANIFEST_DIRECTORY/ingress-classes.yaml" 2>&1 || true
kubectl get configmaps -A -o yaml > "$MANIFEST_DIRECTORY/configmaps.yaml" 2>&1 || true
kubectl describe pods -A > "$MANIFEST_DIRECTORY/pods-describe.txt" 2>&1 || true
kubectl describe deployments -A > "$MANIFEST_DIRECTORY/deployments-describe.txt" 2>&1 || true
kubectl describe ingresses.networking.k8s.io -A > "$MANIFEST_DIRECTORY/ingresses-describe.txt" 2>&1 || true

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
