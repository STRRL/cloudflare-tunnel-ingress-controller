# Design: CloudflareAccess CRD

This document specifies the CloudflareAccess custom resource and its
controller. It is the implementation companion of ADR 0001.

## Purpose

A CloudflareAccess object puts Cloudflare Access authentication in front
of services that this controller already exposes through Ingresses. The
object binds three things together:

- which Ingresses to protect (`targetRefs`)
- which reusable Access Policies apply (`policies`)
- application level settings (session duration, identity providers)

Policy content (who is allowed, denied, which rules) is not managed by
this controller. Users define reusable Access Policies in the Cloudflare
dashboard, Terraform or any other tool, and reference them by name or ID.

## Object model

One CloudflareAccess object owns exactly one Cloudflare Access
Application. All hostnames found in the rules of all referenced
Ingresses become public destinations of that one application. Users
authenticate once and reach every service behind the same
CloudflareAccess object, because all destinations share one application
audience (AUD).

Mapping to the Cloudflare API (cloudflare-go, account scoped):

| CRD | Access Application |
|---|---|
| one object | one application, `type: self_hosted` |
| all hostnames of all targets | `domain` (first hostname) plus `destinations` (all hostnames, type public) |
| `spec.policies`, resolved to IDs | `policies` (list of reusable policy IDs, ascending precedence) |
| `spec.sessionDuration` | `session_duration` |
| `spec.allowedIdentityProviders` | `allowed_idps` |
| `spec.autoRedirectToIdentity` | `auto_redirect_to_identity` |

Identification on the Cloudflare side:

- Application name: `ctic:<tunnelName>:<namespace>/<name>`. The tunnel
  name keeps two clusters that share one Cloudflare account from
  colliding on the same namespace and object name.
- Access tag `managed-by-cloudflare-tunnel-ingress-controller` on every
  managed application. The controller creates the tag on first use. The
  tag marks ownership independently of the display name and makes all
  managed applications visible with one filter in the dashboard.
- Lookup order during reconcile: `status.applicationID` first, fallback
  to listing applications and matching the deterministic name plus the
  ownership tag.

## API

Group `cloudflare-tunnel-ingress-controller.strrl.dev`, version
`v1alpha1`, kind `CloudflareAccess`, namespaced.

```go
type CloudflareAccessSpec struct {
	// TargetRefs selects the Ingresses to protect. Only Ingresses in
	// the same namespace as this object can be referenced. All
	// hostnames of all targets become destinations of one shared
	// Access Application, so one login covers all of them.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	// +listType=map
	// +listMapKey=kind
	// +listMapKey=name
	TargetRefs []TargetRef `json:"targetRefs"`

	// Policies references reusable Access Policies in ascending order
	// of precedence. The first entry is evaluated first.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=32
	Policies []PolicyRef `json:"policies"`

	// SessionDuration is how long a session lasts before the user has
	// to authenticate again.
	// +optional
	// +kubebuilder:validation:Enum=30m;6h;12h;24h;168h;730h
	SessionDuration string `json:"sessionDuration,omitempty"`

	// AllowedIdentityProviders limits which identity providers users
	// can pick for this application, by identity provider ID. Empty
	// means every provider configured in the account.
	// +optional
	// +kubebuilder:validation:MaxItems=16
	AllowedIdentityProviders []string `json:"allowedIdentityProviders,omitempty"`

	// AutoRedirectToIdentity skips the identity provider selection
	// page. The Cloudflare API requires exactly one entry in
	// allowedIdentityProviders when this is true.
	// +optional
	AutoRedirectToIdentity *bool `json:"autoRedirectToIdentity,omitempty"`
}

type TargetRef struct {
	// Kind of the target resource.
	// +kubebuilder:validation:Enum=Ingress
	Kind string `json:"kind"`

	// Name of the target resource.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

type PolicyRef struct {
	// Name of a reusable Access Policy. Resolved to an ID at
	// reconcile time. Exactly one of name and id must be set.
	// +optional
	Name string `json:"name,omitempty"`

	// ID of a reusable Access Policy. Stable across renames. Exactly
	// one of name and id must be set.
	// +optional
	ID string `json:"id,omitempty"`
}

type CloudflareAccessStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ApplicationID of the managed Access Application.
	// +optional
	ApplicationID string `json:"applicationID,omitempty"`

	// AUD is the application audience tag. Origins that validate the
	// Cf-Access-Jwt-Assertion header need this value.
	// +optional
	AUD string `json:"aud,omitempty"`

	// Hostnames currently covered by the Access Application.
	// +optional
	Hostnames []string `json:"hostnames,omitempty"`
}
```

Validation enforced by the API server:

- On the spec:
  `+kubebuilder:validation:XValidation:rule="!(has(self.autoRedirectToIdentity) && self.autoRedirectToIdentity) || (has(self.allowedIdentityProviders) && size(self.allowedIdentityProviders) == 1)",message="autoRedirectToIdentity requires exactly one allowedIdentityProviders entry"`
- On PolicyRef:
  `+kubebuilder:validation:XValidation:rule="has(self.name) != has(self.id)",message="set exactly one of name or id"`
- Duplicate target refs are rejected by the list map keys.

Printer columns:

```go
// +kubebuilder:printcolumn:name="Accepted",type=string,JSONPath=`.status.conditions[?(@.type=="Accepted")].status`
// +kubebuilder:printcolumn:name="Application",type=string,JSONPath=`.status.applicationID`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
```

Example:

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
    - name: ci-tokens
  sessionDuration: 24h
```

## Reconciliation

A dedicated controller reconciles CloudflareAccess objects. It watches:

- CloudflareAccess objects (primary)
- Ingresses, mapped back to referencing CloudflareAccess objects
  through a field index on `spec.targetRefs`

Reconcile steps:

1. Load the object. If it is being deleted, run cleanup (below).
2. Resolve targets: fetch every referenced Ingress in the same
   namespace, collect hostnames from all rules, deduplicate. Wildcard
   hostnames pass through unchanged, Cloudflare accepts them.
3. Detect conflicts: if any hostname is already covered by another
   CloudflareAccess object, the oldest object by creation timestamp
   (ties broken by namespace/name order) wins and keeps working. Every
   younger conflicting object gets `Accepted: False` with reason
   `Conflicted` and is not reconciled further.
4. Resolve policies: list the account's reusable Access Policies once.
   Resolve every PolicyRef to an ID. A name that matches nothing fails
   with reason `PolicyNotFound`; a name that matches more than one
   policy fails with reason `AmbiguousPolicy`; an ID that does not
   exist fails with reason `PolicyNotFound`.
5. Ensure the ownership tag exists.
6. Create or update the Access Application with the full desired state
   (idempotent update, drift on the Cloudflare side gets corrected).
7. Write status: `applicationID`, `aud`, `hostnames`, conditions,
   `observedGeneration`. Requeue after 10 minutes to correct external
   drift.

Fail closed. Whenever target or policy resolution fails, the controller
reports the failure in conditions and stops. It never applies a partial
policy list and it never deletes the Access Application because a
reference stopped resolving. An application is deleted only when its
CloudflareAccess object is deleted.

Special cases:

- Zero hostnames after target resolution (targets missing or targets
  without rules): `ResolvedRefs: False`, reason `NoHostnames`, existing
  application stays untouched.
- A referenced Ingress disappears while others remain: the application
  shrinks to the remaining hostnames, `ResolvedRefs: False` with reason
  `TargetNotFound` reports the missing one.

Deletion runs behind the finalizer
`cloudflare-tunnel-ingress-controller.strrl.dev/access-cleanup`: delete
the Access Application, then remove the finalizer. If the application
is already gone, removal succeeds silently.

## Conditions

| Type | False reasons | Meaning |
|---|---|---|
| `Accepted` | `Conflicted`, `CloudflareError` | The object is active and its application is being managed |
| `ResolvedRefs` | `TargetNotFound`, `NoHostnames`, `PolicyNotFound`, `AmbiguousPolicy` | Every reference in the spec resolved |

## Interaction with the existing sync loop

The existing exposure sync (tunnel ingress rules, DNS) stays untouched.
Access reconciliation is a separate controller with its own Cloudflare
API calls. Installations that never create a CloudflareAccess object
never trigger an Access API call, so their API tokens do not need
Access permissions. Users of the CRD need a token that additionally has
the Access: Apps and Policies Edit permission; the reference
documentation lists it next to the existing token permissions.

## Packaging

- The CRD manifest is generated by controller-gen and committed. CI
  fails when the committed manifest differs from the generated one.
- The Helm chart ships the CRD as a template (not the crds directory,
  which Helm never upgrades) with `helm.sh/resource-policy: keep`, so
  chart upgrades update the schema and chart uninstall leaves the CRD
  and its objects in place.
- Uninstall order is documented: delete CloudflareAccess objects while
  the controller still runs, then uninstall the chart. Otherwise the
  finalizer blocks namespace deletion until removed by hand.
- The controller RBAC gains rules for `cloudflareaccesses`,
  `cloudflareaccesses/status` and `cloudflareaccesses/finalizers`.

## Code layout

- `pkg/apis/cloudflareaccess/v1alpha1/`: types with kubebuilder
  markers, deepcopy generated by controller-gen.
- `pkg/controller/access-controller.go`: the reconciler and the
  Ingress index.
- `pkg/cloudflare-controller/access.go`: Cloudflare client logic:
  policy resolution, tag handling, application create/update/delete.
- Makefile targets `generate` and `manifests`; both run in CI.

## Observability

- Kubernetes Events on the CloudflareAccess object for create, update,
  delete and every failed resolution.
- Metrics follow the existing `pkg/metrics` conventions: a gauge for
  managed applications, a counter for Access API errors.

## Out of scope

- Managing policy content, Access Groups or identity providers.
- Application types other than `self_hosted` (ssh, vnc, saas,
  infrastructure).
- Hostname subsetting per target. If it turns out to be needed, an
  optional `hostnames` allow list can be added to the spec as a
  compatible change.
- Label selector targeting. Explicit references only; a selector can
  be added later without breaking anything.

## Future

- Gateway API: when HTTPRoute support lands, `TargetRef.Kind` gains an
  `HTTPRoute` enum value and user manifests stay unchanged.
- An optional convenience annotation on Ingresses that expands to a
  generated CloudflareAccess object may be added later if users ask
  for a shorter form.
