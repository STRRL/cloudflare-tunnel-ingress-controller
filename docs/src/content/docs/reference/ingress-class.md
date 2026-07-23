---
title: Ingress Class
description: Understand how the controller uses the cloudflare-tunnel ingress class.
---

The Helm chart installs an `IngressClass` named `cloudflare-tunnel` that the controller watches. Any `Ingress` tagged with this class is reconciled into Cloudflare Tunnel and DNS records.

## Default behaviour

| Field                          | Default                                          | Description                                                                    |
| ------------------------------ | ------------------------------------------------ | ------------------------------------------------------------------------------ |
| `ingressClass.name`            | `cloudflare-tunnel`                              | Name of the class to reference from your ingress objects.                      |
| `ingressClass.controllerValue` | `strrl.dev/cloudflare-tunnel-ingress-controller` | Identifier reported back to Kubernetes.                                        |
| `ingressClass.isDefaultClass`  | `false`                                          | When set to `true`, new ingresses without a class inherit `cloudflare-tunnel`. |

To target the controller, set one of the following on your ingress manifest:

```yaml
spec:
  ingressClassName: cloudflare-tunnel
```

or the legacy annotation:

```yaml
metadata:
  annotations:
    kubernetes.io/ingress.class: cloudflare-tunnel
```

Avoid enabling the class globally (`isDefaultClass: true`) unless every ingress in the cluster should use Cloudflare Tunnel. Mixing controllers with the same default class can create conflicting reconciliations.
