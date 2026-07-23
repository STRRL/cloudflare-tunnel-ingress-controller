---
title: Ingress
description: Configure ingress resources and annotations for Cloudflare Tunnel.
---

This guide explains how to configure Ingress resources for Cloudflare Tunnel.

For general information about Kubernetes Ingress resources, see the [official Kubernetes Ingress documentation](https://kubernetes.io/docs/concepts/services-networking/ingress/).

For detailed configuration options specific to Cloudflare Tunnel, refer to the [ingress annotations reference](/reference/ingress-annotations/).

Each ingress assigned to the `cloudflare-tunnel` class becomes a Cloudflare route. The controller provisions DNS records and launches `cloudflared` connectors that proxy traffic back to your Service.

## Required fields

| Field                                            | Description                                      |
| ------------------------------------------------ | ------------------------------------------------ |
| `spec.rules[].host`                              | External hostname to publish via Cloudflare DNS. |
| `spec.rules[].http.paths[].backend.service.name` | Target Kubernetes Service name.                  |
| `spec.rules[].http.paths[].backend.service.port` | Service port number or name to proxy.            |

Example manifest:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dashboard
  namespace: kubernetes-dashboard
  annotations:
    kubernetes.io/ingress.class: cloudflare-tunnel
spec:
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

Consult the [ingress annotations reference](/reference/ingress-annotations/) for advanced routing behaviour such as protocol overrides, TLS verification settings, and host header rewrites.

## Wildcard hostnames

Hosts may use a leading wildcard label such as `*.example.com`. The controller creates the matching wildcard DNS record and orders the tunnel rules so that routing behaves as you would expect:

- An exact hostname always wins over a wildcard. With rules for `app.example.com` and `*.example.com`, requests to `app.example.com` reach the exact rule and any other subdomain falls back to the wildcard.
- A more specific wildcard wins over a broader one. `*.internal.example.com` is matched before `*.example.com`.
- For the same hostname, rules with longer paths are matched first.

The order of rules inside your Ingress spec does not matter. The controller sorts them deterministically before writing the tunnel configuration.

## Troubleshooting with events

The Ingress API has no status conditions, so the controller reports problems as Warning events on the Ingress object. Check them with `kubectl describe ingress <name>`:

| Reason            | Meaning                                                                                                          |
| ----------------- | ---------------------------------------------------------------------------------------------------------------- |
| `RuleSkipped`     | A rule carries only a host and no `http` section, so it cannot be turned into a tunnel route.                    |
| `TLSIgnored`      | The ingress has a `tls` section. SSL passthrough is not supported, Cloudflare terminates TLS at the edge.        |
| `TransformFailed` | A rule could not be transformed, for example a headless Service, a missing Service, or an unsupported `pathType`. |

Rules that fail to transform are skipped while the remaining rules still reconcile, so a single broken rule does not take down the other routes of the ingress.
