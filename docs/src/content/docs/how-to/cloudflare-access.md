---
title: Protect Services with Cloudflare Access
description: Put Cloudflare Access authentication in front of exposed services with the CloudflareAccess custom resource.
---

The `CloudflareAccess` custom resource puts Cloudflare Access
authentication in front of Ingresses that this controller exposes. The
controller creates and manages one Access Application per
`CloudflareAccess` object; the rules about who may pass stay in
reusable Access Policies that you manage in the Cloudflare dashboard or
with any other tool.

This feature is alpha. The API group is `v1alpha1` and fields may
change between releases.

## Prerequisites

1. At least one reusable Access Policy in your Cloudflare account.
   Create policies under `Zero Trust -> Access -> Policies`. Only
   reusable policies can be referenced; policies embedded in one
   application cannot.
2. The controller API token needs one more permission scope:
   `Account -> Access: Apps and Policies -> Edit`. Installations that
   never create a `CloudflareAccess` object do not need it.

## Protect one or more Ingresses

Create a `CloudflareAccess` object in the same namespace as the
Ingresses it protects:

```yaml
apiVersion: cloudflare-tunnel-ingress-controller.strrl.dev/v1alpha1
kind: CloudflareAccess
metadata:
  name: monitoring-access
  namespace: monitoring
spec:
  targetRefs:
    - kind: Ingress
      name: grafana
    - kind: Ingress
      name: prometheus
  policies:
    - name: internal-only
  sessionDuration: 24h
```

Every hostname of every referenced Ingress becomes a destination of one
shared Access Application, so one login covers all of them. Policies
are referenced by `name` or by `id` and are attached in list order; the
first entry is evaluated first.

Optional application settings:

```yaml
spec:
  sessionDuration: 6h
  allowedIdentityProviders:
    - 6a1e6a2b-0000-0000-0000-000000000000
  autoRedirectToIdentity: true
```

`autoRedirectToIdentity` skips the identity provider selection page and
requires exactly one entry in `allowedIdentityProviders`.

## Check the result

```bash
kubectl get cloudflareaccess -n monitoring
```

The `Accepted` column shows whether the object is active. Details live
in the status:

```bash
kubectl describe cloudflareaccess monitoring-access -n monitoring
```

1. The `Accepted` condition is false when another `CloudflareAccess`
   object already covers one of the hostnames (the oldest object wins)
   or when the Cloudflare API rejected a change.
2. The `ResolvedRefs` condition is false when a referenced Ingress or
   policy does not resolve. The controller then leaves the existing
   Access Application untouched instead of applying a partial
   configuration, so a typo can never remove authentication.
3. `status.aud` holds the application audience tag, needed when your
   origin validates the `Cf-Access-Jwt-Assertion` header.

## Remove protection

Delete the `CloudflareAccess` object. A finalizer makes the controller
delete the Access Application before the object goes away.

Delete all `CloudflareAccess` objects before you uninstall the
controller. Without a running controller the finalizer stays on the
objects and blocks their deletion until you remove it by hand.
