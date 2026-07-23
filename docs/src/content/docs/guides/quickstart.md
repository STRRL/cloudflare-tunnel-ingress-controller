---
title: Quickstart
description: Install the Cloudflare Tunnel Ingress Controller and expose your first service.
---

Use this guide to install the controller with Helm and publish a Kubernetes Service through Cloudflare Tunnel using a standard `Ingress` resource.

## Prerequisites

- A Kubernetes cluster running version 1.26 or later with cluster-admin access.
- `kubectl` and `helm` configured for the cluster.
- A Cloudflare account with an active zone and Argo Tunnel access enabled.
- An API token that can manage tunnels and DNS:
  - `Account.Cloudflare Tunnel:Edit`
  - `Zone.DNS:Edit`
  - `Zone.Zone:Read`
- You can create API Key and prefill the permissions with [this template](https://dash.cloudflare.com/profile/api-tokens?permissionGroupKeys=%5B%7B%22key%22%3A%22zone%22%2C%22type%22%3A%22read%22%7D%2C%7B%22key%22%3A%22dns%22%2C%22type%22%3A%22edit%22%7D%2C%7B%22key%22%3A%22argotunnel%22%2C%22type%22%3A%22edit%22%7D%5D&name=Cloudflare%20Tunnel%20Ingress%20Controller&accountId=*&zoneId=all).
- Your Cloudflare account ID. Follow the official guide to [find your account and zone IDs](https://developers.cloudflare.com/fundamentals/get-started/basic-tasks/find-account-and-zone-ids/).

## 1. Add the Helm repository

Add the official chart and refresh your local index:

```bash
helm repo add strrl.dev https://helm.strrl.dev
helm repo update
```

## 2. Install the controller

Install (or upgrade) the controller. Replace the placeholders with your API token, account ID, and preferred tunnel name. The chart provisions the `cloudflare-api` secret automatically using these values.

```bash
helm upgrade --install --wait \
  cloudflare-tunnel-ingress-controller \
  strrl.dev/cloudflare-tunnel-ingress-controller \
  --namespace cloudflare-tunnel-ingress-controller --create-namespace \
  --set cloudflare.apiToken="<CLOUDFLARE_API_TOKEN>" \
         cloudflare.accountId="<CLOUDFLARE_ACCOUNT_ID>" \
         cloudflare.tunnelName="<TUNNEL_NAME>"
```

Verify the controller pod and the bundled `cloudflared` connector are running:

```bash
kubectl get pods -n cloudflare-tunnel-ingress-controller
```

## 3. Publish a Service with Ingress

Create an `Ingress` that targets your Service and assigns the `cloudflare-tunnel` ingress class. The controller watches for these routes and configures Cloudflare automatically.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dashboard
  namespace: kubernetes-dashboard
spec:
  ingressClassName: cloudflare-tunnel
  rules:
    - host: dash.example.com # <- REPLACE ME!
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: kubernetes-dashboard
                port:
                  number: 80
```

Apply the manifest and monitor the ingress status until a Cloudflare hostname appears:

```bash
kubectl apply -f dashboard-ingress.yaml
kubectl get ingress dashboard -n kubernetes-dashboard -o yaml
```

## 4. Validate externally

- Visit `https://dash.example.com` (or your chosen hostname) to confirm the proxied application is reachable.
- Run `kubectl logs deployment/cloudflare-tunnel-ingress-controller -n cloudflare-tunnel-ingress-controller` to troubleshoot tunnel or DNS issues.

## Next steps

- Review the reference docs for the [ingress class](/reference/ingress-class/), [credentials](/reference/cloudflare-credentials/), [ingress](/reference/ingress/), and [ingress annotations](/reference/ingress-annotations/).
- Switch the chart to an existing secret if you prefer to manage credentials outside Helm releases.
- Automate deployment via GitOps and monitor the `cloudflared` connector pods for long-lived tunnels.
