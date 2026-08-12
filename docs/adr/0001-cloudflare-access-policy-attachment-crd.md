# ADR 0001: Cloudflare Access integration through a policy attachment CRD

Date: 2026-08-12

Status: Accepted

## Context

Users who expose internal services through this controller usually want
authentication in front of them. Cloudflare answers this need with
Cloudflare Access: an Access Application is created for a hostname, and
Access Policies on that application decide who may pass.

Today the controller only manages tunnel ingress rules and DNS records.
Access Applications must be managed somewhere else (dashboard, Terraform,
another operator), so every service change touches two systems. Issue #83
asks for the controller to manage Access Applications from the Ingress.
Pull requests #277 and #278 implemented this with Ingress annotations that
inline the policy content (allowed groups, denied groups, bypass, session
duration, auto redirect).

Community demand is real: #83 has multiple reactions, a dedicated operator
(cloudflare-zero-trust-operator) exists only to manage Access objects from
Kubernetes, and the largest tunnel operator has been forked to add Access
support.

We reviewed the annotation design and found problems we do not want to
freeze into a public API:

1. Expressiveness ceiling. Access policy rules cover emails, email
   domains, IP ranges, countries, service tokens, mTLS and device
   posture. Flat string annotations cannot carry this structure, so the
   annotation list would keep growing, one annotation per rule type.
2. No validation and no feedback. Annotations are plain strings. A typo
   in a group name cannot be rejected at apply time and there is no
   status field to report it later. In the reviewed implementation a typo
   silently removed the Access Application and left the service exposed
   without authentication (fail open).
3. Conflict with the Gateway API future. Issue #232 tracks Gateway API
   support. There, authentication settings belong in a policy attachment
   object (GEP 713), not in annotations. Shipping a wide annotation
   surface now creates a legacy API we would have to carry through that
   migration.

## Decision

We integrate Cloudflare Access through a namespaced CRD that follows the
policy attachment pattern, and we keep policy content out of the
controller.

```yaml
apiVersion: cloudflare-tunnel-ingress-controller.strrl.dev/v1alpha1
kind: CloudflareAccess
metadata:
  name: grafana-access
  namespace: monitoring
spec:
  targetRefs:
    - kind: Ingress
      name: grafana
  policies:
    - internal-only
    - ci-tokens
  sessionDuration: 1h
  autoRedirect: true
status:
  conditions:
    - type: Accepted
      status: "True"
```

Division of responsibility:

- The controller owns the lifecycle of Access Applications: create one
  per hostname of each referenced Ingress, keep it in sync, delete it
  when the Ingress goes away or the reference is removed. Only the
  controller can do this well, because only it knows the Ingress
  lifecycle.
- The controller does not own policy content. `spec.policies` lists the
  names of reusable Access Policies that the user defines in the
  Cloudflare dashboard, Terraform or any other tool. The controller only
  resolves the names and attaches the policies to the application.

Key semantics (the full specification lives in
`docs/design/cloudflare-access.md`):

- One CloudflareAccess object owns exactly one Access Application. All
  hostnames of all referenced Ingresses become destinations of that
  application, so one login covers all of them.
- `targetRefs` may only reference Ingresses in the same namespace as the
  CRD. This follows GEP 713 and prevents one namespace from attaching
  policies to another namespace's services.
- Fail closed. If a referenced reusable policy does not exist, the
  controller reports the failure in status and does not touch the
  existing Access Application. It never deletes an application because
  a name failed to resolve.
- Conflicts resolve by age. When two CloudflareAccess objects cover the
  same hostname, the oldest object wins and every younger one gets a
  `Conflicted` condition and is not reconciled.
- Ownership on the Cloudflare side uses an Access tag plus a
  deterministic application name `ctic:<tunnelName>:<namespace>/<name>`,
  the same idea as the `_ctic_managed` TXT records for DNS.
- The API group starts at `v1alpha1`. Field changes are allowed until it
  graduates.

## Alternatives considered

1. Annotations that inline policy content (pull requests #277 and #278).
   Rejected for the reasons in the context section: expressiveness
   ceiling, no validation, no status, fail open failure modes, and a
   legacy surface for the Gateway API migration.
2. A single annotation that references reusable policy names. Closest to
   the one line spirit of the project and free of the expressiveness
   problem, but still no schema validation, no status conditions, no
   RBAC separation, and annotations cannot follow the move to HTTPRoute.
   May return later as sugar on top of the CRD if users ask for it.
3. Recommend an external operator and do nothing. Keeps the controller
   small, but every service change keeps touching two systems, which is
   the exact pain #83 describes, and the external operators do not know
   the Ingress lifecycle.

## Consequences

- The Helm chart starts to install and manage a CRD, including upgrade
  handling and a decision about keeping the CRD on uninstall.
- The controller watches a second resource type and reconciles the join
  of CloudflareAccess and Ingress objects.
- The API token needs Access permissions only for users who create
  CloudflareAccess objects. Installations that never use the CRD must
  work with a token that has no Access permissions, so Access
  reconciliation runs only when CloudflareAccess objects exist.
- When Gateway API support (#232) lands, `targetRefs.kind: HTTPRoute` is
  added and user manifests keep working unchanged.
- Pull requests #277 and #278 are closed in favor of this design. The
  ownership naming and the reconcile structure from that work carry over
  into the implementation.
