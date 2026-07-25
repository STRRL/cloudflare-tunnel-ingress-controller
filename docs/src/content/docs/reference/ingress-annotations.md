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

## Origin request settings

These annotations map to cloudflared `originRequest` settings and apply to every rule generated from the ingress. Omitted annotations keep the cloudflared defaults, with one historical exception: for `backend-protocol: https` the controller disables TLS verification unless told otherwise, so enable verification explicitly with `no-tls-verify: "false"` (or the legacy `proxy-ssl-verify: "on"`). Durations are Go duration strings in whole seconds, such as `30s` or `2m`. See the upstream [origin configuration parameters](https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/configure-tunnels/origin-parameters/) reference for the behaviour of each setting.

| Annotation                                                                  | Purpose                                                                                                                  |
| ---------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| `cloudflare-tunnel-ingress-controller.strrl.dev/connect-timeout`            | Timeout for establishing a new TCP connection to the origin.                                                             |
| `cloudflare-tunnel-ingress-controller.strrl.dev/tls-timeout`                | Timeout for completing a TLS handshake with the origin.                                                                  |
| `cloudflare-tunnel-ingress-controller.strrl.dev/tcp-keepalive`              | TCP keepalive interval for connections to the origin.                                                                    |
| `cloudflare-tunnel-ingress-controller.strrl.dev/no-happy-eyeballs`          | Set to `"true"` to disable the IPv4/IPv6 fallback when connecting to the origin.                                         |
| `cloudflare-tunnel-ingress-controller.strrl.dev/keepalive-connections`      | Maximum keepalive connection pool size towards the origin.                                                               |
| `cloudflare-tunnel-ingress-controller.strrl.dev/keepalive-timeout`          | Timeout for closing idle connections to the origin.                                                                      |
| `cloudflare-tunnel-ingress-controller.strrl.dev/no-tls-verify`              | Set to `"true"` to disable TLS certificate verification of the origin. Mutually exclusive with `proxy-ssl-verify`.       |
| `cloudflare-tunnel-ingress-controller.strrl.dev/disable-chunked-encoding`   | Set to `"true"` to disable chunked transfer encoding towards the origin, useful for WSGI servers.                        |
| `cloudflare-tunnel-ingress-controller.strrl.dev/http2-origin`               | Set to `"true"` to connect to the origin with HTTP/2. Requires `backend-protocol: https`, HTTP/2 needs TLS.              |

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
