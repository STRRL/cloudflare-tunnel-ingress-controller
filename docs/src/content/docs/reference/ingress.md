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
