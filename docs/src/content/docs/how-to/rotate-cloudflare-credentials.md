---
title: Rotate Cloudflare Credentials
description: Replace the controller API token and restart workloads that consume it.
---

Rotate the Cloudflare API token by updating its Kubernetes Secret and restarting the controller. The controller reads its credential environment variables only at startup.

Review [Cloudflare credentials](/reference/cloudflare-credentials/) before starting. The replacement token needs the required permissions listed there.

## 1. Create the replacement token

Create a new Cloudflare API token with the same account, zone scope, and required permissions as the current token. Keep the current token valid until rotation is complete.

## 2. Update the credential source

Choose the method used by your Helm release.

If Helm creates the `cloudflare-api` Secret, update `cloudflare.apiToken` in the secure values source used for the release, then run your normal Helm upgrade.

If `cloudflare.secretRef` points to an existing Secret, update the key selected by `cloudflare.secretRef.apiTokenKey`. When an external secret controller owns that Secret, update the external provider and wait for the new value to sync into Kubernetes.

Check that Kubernetes recorded a new Secret version without printing its contents:

```bash
kubectl get secret <SECRET_NAME> \
  -n <CONTROLLER_NAMESPACE> \
  -o jsonpath='{.metadata.resourceVersion}{"\n"}'
```

## 3. Restart the controller

Restart the controller Deployment so every replica reads the replacement token:

```bash
kubectl rollout restart deployment <CONTROLLER_DEPLOYMENT> \
  -n <CONTROLLER_NAMESPACE>
```

Wait for the rollout:

```bash
kubectl rollout status deployment <CONTROLLER_DEPLOYMENT> \
  -n <CONTROLLER_NAMESPACE>
```

## 4. Verify Cloudflare reconciliation

Check the new controller pods:

```bash
kubectl logs deployment/<CONTROLLER_DEPLOYMENT> \
  -n <CONTROLLER_NAMESPACE> \
  --since=10m
```

Confirm the controller can bootstrap the configured tunnel and reconcile existing Ingress resources without authentication errors.

The elected controller also fetches the tunnel token for the managed `cloudflared` connector. If that token changes, the controller updates the `controlled-cloudflared-token` Secret and rolls the connector Deployment through its pod template annotation.

Check connector availability:

```bash
kubectl rollout status deployment controlled-cloudflared-connector \
  -n <CONTROLLER_NAMESPACE>
```

## 5. Revoke the old token

Revoke the previous Cloudflare API token only after controller reconciliation and connector availability are healthy.
