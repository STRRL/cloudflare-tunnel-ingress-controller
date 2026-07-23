---
title: Helm Values
description: Tune the controller and cloudflared connectors with chart values.
---

The `strrl.dev/cloudflare-tunnel-ingress-controller` chart exposes values for production hardening, observability, and connector behaviour. This table highlights the most frequently adjusted settings.

For the complete and up-to-date list of all available Helm values, refer to the [values.yaml](https://github.com/STRRL/cloudflare-tunnel-ingress-controller/blob/master/helm/cloudflare-tunnel-ingress-controller/values.yaml) file in the repository.

| Value                              | Default                             | Notes                                                                           |
| ---------------------------------- | ----------------------------------- | ------------------------------------------------------------------------------- |
| `cloudflare.apiToken`              | `""`                                | Required when Helm creates the credential secret.                               |
| `cloudflare.accountId`             | `""`                                | Required when Helm creates the credential secret.                               |
| `cloudflare.tunnelName`            | `""`                                | Required when Helm creates the credential secret.                               |
| `cloudflare.secretRef.*`           | unset                               | Point the chart at an existing secret managed outside Helm.                     |
| `ingressClass.name`                | `cloudflare-tunnel`                 | Rename if you run multiple controllers in one cluster.                          |
| `ingressClass.isDefaultClass`      | `false`                             | Set to `true` only if Cloudflare Tunnel should handle every ingress by default. |
| `cloudflared.image.tag`            | `latest`                            | Pin to a specific cloudflared version for reproducible environments.            |
| `cloudflared.extraArgs`            | `[]`                                | Append extra arguments such as `--post-quantum`.                                |
| `cloudflaredServiceMonitor.create` | `false`                             | Enable when you scrape metrics with Prometheus Operator.                        |
| `replicaCount`                     | `1`                                 | Scale the controller deployment (enable leader election for >1).                |
| `resources`                        | requests/limits at 100m CPU / 128Mi | Harden resource guarantees for both the controller and connectors.              |

Most other values control standard Kubernetes objects (service account, annotations, node selectors, tolerations, affinity). Use them to integrate with your platform’s scheduling or security requirements.
