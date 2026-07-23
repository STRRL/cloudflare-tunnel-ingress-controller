---
title: Ingress Annotations
description: Fine-tune Cloudflare Tunnel behaviour with controller-specific annotations.
---

Annotations let you customise how the controller configures Cloudflare for each ingress rule. Apply them on the ingress metadata alongside the `cloudflare-tunnel` class.

| Annotation                                                          | Purpose                                                                            |
| ------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| `cloudflare-tunnel-ingress-controller.strrl.dev/backend-protocol`   | Override the upstream protocol (`http` default, `https` supported).                |
| `cloudflare-tunnel-ingress-controller.strrl.dev/proxy-ssl-verify`   | Enable (`on`) or disable (`off`) TLS verification when proxying to HTTPS backends. |
| `cloudflare-tunnel-ingress-controller.strrl.dev/http-host-header`   | Rewrite the HTTP Host header sent to the backend Service.                          |
| `cloudflare-tunnel-ingress-controller.strrl.dev/origin-server-name` | Set the SNI hostname when terminating TLS to the origin.                           |

Example ingress snippet:

```yaml
metadata:
  name: dashboard
  namespace: kubernetes-dashboard
  annotations:
    kubernetes.io/ingress.class: cloudflare-tunnel
    cloudflare-tunnel-ingress-controller.strrl.dev/backend-protocol: https
    cloudflare-tunnel-ingress-controller.strrl.dev/proxy-ssl-verify: on
    cloudflare-tunnel-ingress-controller.strrl.dev/http-host-header: dash.internal.svc
    cloudflare-tunnel-ingress-controller.strrl.dev/origin-server-name: dash.internal.svc
```

The controller emits Kubernetes events and logs when an annotation is invalid or conflicts with connector capabilities. Tail controller logs to confirm annotated rules reconcile successfully.
