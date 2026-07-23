---
title: Ingress Annotations
description: Fine-tune Cloudflare Tunnel behaviour with controller-specific annotations.
---

Annotations let you customise how the controller configures Cloudflare for each ingress rule. Apply them on the ingress metadata alongside the `cloudflare-tunnel` class.

| Annotation                                                              | Purpose                                                                                                                                               |
| ----------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| `cloudflare-tunnel-ingress-controller.strrl.dev/backend-protocol`       | Protocol used to reach the backend Service (`http` default). Any protocol supported by cloudflared works, including `https`, `tcp`, `ssh`, and `rdp`. |
| `cloudflare-tunnel-ingress-controller.strrl.dev/proxy-ssl-verify`       | Enable (`on`) or disable (`off`) TLS verification when proxying to HTTPS backends.                                                                    |
| `cloudflare-tunnel-ingress-controller.strrl.dev/http-host-header`       | Rewrite the HTTP Host header sent to the backend Service.                                                                                             |
| `cloudflare-tunnel-ingress-controller.strrl.dev/origin-server-name`     | Set the SNI hostname when terminating TLS to the origin.                                                                                              |
| `cloudflare-tunnel-ingress-controller.strrl.dev/disable-dns-management` | Set to `"true"` to stop the controller from managing Cloudflare DNS records for this ingress while still configuring the tunnel route.                |

Example Ingress snippet:

```yaml
metadata:
  name: dashboard
  namespace: kubernetes-dashboard
  annotations:
    cloudflare-tunnel-ingress-controller.strrl.dev/backend-protocol: https
    cloudflare-tunnel-ingress-controller.strrl.dev/proxy-ssl-verify: "on"
    cloudflare-tunnel-ingress-controller.strrl.dev/http-host-header: dash.internal.svc
    cloudflare-tunnel-ingress-controller.strrl.dev/origin-server-name: dash.internal.svc
spec:
  ingressClassName: cloudflare-tunnel
```

For task focused examples, see [Expose non HTTP services](/how-to/expose-non-http-services/) and [Use an external DNS system](/how-to/use-with-external-dns/).

## Validation feedback

The controller emits Kubernetes Warning events on the Ingress object when a rule is invalid or cannot be applied, visible via `kubectl describe ingress`. See [troubleshooting with events](/reference/ingress/#troubleshooting-with-events) for the event reasons and their meaning.
