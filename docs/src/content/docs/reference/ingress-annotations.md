---
title: Ingress Annotations
description: Fine-tune Cloudflare Tunnel behaviour with controller-specific annotations.
---

Annotations let you customise how the controller configures Cloudflare for each ingress rule. Apply them on the ingress metadata alongside the `cloudflare-tunnel` class.

| Annotation                                                              | Purpose                                                                                                |
| ----------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| `cloudflare-tunnel-ingress-controller.strrl.dev/backend-protocol`       | Protocol used to reach the backend Service (`http` default). Any protocol supported by cloudflared works, including `https`, `tcp`, `ssh`, and `rdp`. |
| `cloudflare-tunnel-ingress-controller.strrl.dev/proxy-ssl-verify`       | Enable (`on`) or disable (`off`) TLS verification when proxying to HTTPS backends.                     |
| `cloudflare-tunnel-ingress-controller.strrl.dev/http-host-header`       | Rewrite the HTTP Host header sent to the backend Service.                                              |
| `cloudflare-tunnel-ingress-controller.strrl.dev/origin-server-name`     | Set the SNI hostname when terminating TLS to the origin.                                               |
| `cloudflare-tunnel-ingress-controller.strrl.dev/disable-dns-management` | Set to `"true"` to stop the controller from managing Cloudflare DNS records for this ingress while still configuring the tunnel route. |

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

## Non HTTP backend protocols

Set `backend-protocol` to `tcp`, `ssh`, `rdp`, or any other protocol cloudflared supports to expose non HTTP services. The Kubernetes Ingress spec still requires an `http.paths` entry for such rules, but the controller drops the path from the resulting tunnel rule because path based routing only exists for HTTP(S). Use `/` as a placeholder path.

## Disabling DNS management

Set `disable-dns-management: "true"` when DNS for a hostname is delegated to another system, for example external-dns or a Cloudflare Load Balancer in front of the tunnel. The controller keeps reconciling the tunnel ingress rule but no longer creates or updates the CNAME and ownership TXT records, and it stops erroring when the hostname belongs to no managed zone.

If the controller previously managed DNS for the hostname, enabling the annotation makes it relinquish ownership instead of just going hands off:

- The ownership TXT record it created is deleted.
- Its CNAME record is deleted only while it still points at this tunnel. A CNAME already repointed by an external system is left untouched.
- Records owned by a different tunnel are never touched.

## Validation feedback

The controller emits Kubernetes Warning events on the Ingress object when a rule is invalid or cannot be applied, visible via `kubectl describe ingress`. See [troubleshooting with events](/reference/ingress/#troubleshooting-with-events) for the event reasons and their meaning.
