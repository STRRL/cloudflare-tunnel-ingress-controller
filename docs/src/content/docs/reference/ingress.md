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

See the [Quickstart Ingress example](/guides/quickstart/#3-publish-a-service-with-ingress) for a complete dashboard manifest. Set `spec.ingressClassName` or the legacy `kubernetes.io/ingress.class` annotation to `cloudflare-tunnel`, then provide the host, backend Service, and port described above.

Consult the [ingress annotations reference](/reference/ingress-annotations/) for advanced routing behaviour such as protocol overrides, TLS verification settings, and host header rewrites.

## Wildcard hostnames

Hosts may use a leading wildcard label such as `*.example.com`. The controller creates the matching wildcard DNS record and orders the tunnel rules so that routing behaves as you would expect:

- An exact hostname always wins over a wildcard. With rules for `app.example.com` and `*.example.com`, requests to `app.example.com` reach the exact rule and any other subdomain falls back to the wildcard.
- A more specific wildcard wins over a broader one. `*.internal.example.com` is matched before `*.example.com`.
- For the same hostname, rules with longer paths are matched first.

The order of rules inside your Ingress spec does not matter. The controller sorts them deterministically before writing the tunnel configuration.

## Troubleshooting

See the [troubleshooting guide](/guides/troubleshooting/) for Ingress warning events, log commands, and common DNS and tunnel problems.
