---
title: Quickstart
description: Install the Cloudflare Tunnel Ingress Controller and expose your first service.
---

Use this guide to install the controller with Helm and publish a Kubernetes Service through Cloudflare Tunnel using a standard `Ingress` resource.

## Prerequisites

- A Kubernetes cluster running version 1.26 or later with cluster-admin access.
- `kubectl` and `helm` configured for the cluster.
- A Cloudflare account with an active zone and Argo Tunnel access enabled.
- A Cloudflare API token with `Account.Cloudflare Tunnel:Edit`, `Zone.DNS:Edit`, and `Zone.Zone:Read` permissions. Create it quickly using [this template](https://dash.cloudflare.com/profile/api-tokens?permissionGroupKeys=%5B%7B%22key%22%3A%22zone%22%2C%22type%22%3A%22read%22%7D%2C%7B%22key%22%3A%22dns%22%2C%22type%22%3A%22edit%22%7D%2C%7B%22key%22%3A%22argotunnel%22%2C%22type%22%3A%22edit%22%7D%5D&name=Cloudflare%20Tunnel%20Ingress%20Controller&accountId=*&zoneId=all), or see [Cloudflare credentials](/reference/cloudflare-credentials/) for details.
- Your Cloudflare account ID.
- A Service named `kubernetes-dashboard` in the `kubernetes-dashboard` namespace.

## 1. Install the controller

Replace the placeholders with your API token, account ID, and preferred tunnel name. Helm installs the controller and creates its credential Secret.

```bash
helm upgrade --install --wait \
  cloudflare-tunnel-ingress-controller \
  cloudflare-tunnel-ingress-controller \
  --repo https://helm.strrl.dev \
  --namespace cloudflare-tunnel-ingress-controller \
  --create-namespace \
  --set cloudflare.apiToken="<CLOUDFLARE_API_TOKEN>" \
  --set cloudflare.accountId="<CLOUDFLARE_ACCOUNT_ID>" \
  --set cloudflare.tunnelName="<TUNNEL_NAME>"
```

## 2. Create your first Ingress

Save the following manifest as `dashboard-ingress.yaml`. Replace `dash.example.com` with a hostname in your Cloudflare zone.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dashboard
  namespace: kubernetes-dashboard
spec:
  ingressClassName: cloudflare-tunnel
  rules:
    - host: dash.example.com
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

Apply the manifest:

```bash
kubectl apply -f dashboard-ingress.yaml
```

## 3. Verify the Ingress

Open `https://dash.example.com`, using the hostname you chose. The Kubernetes Dashboard should load through Cloudflare Tunnel.

If something goes wrong, see [troubleshooting](/guides/troubleshooting/).
