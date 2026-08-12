# ADR 0002: Gateway API support with an in cluster data plane, driven by the conformance suite

Date: 2026-08-12

Status: Accepted

## Context

Issue #232 tracks Gateway API support. The upstream Ingress API is
frozen and Gateway API is its successor, so the question is how, not
whether.

The concepts of this controller map onto Gateway API cleanly on the
control plane side:

- GatewayClass with `spec.controllerName` replaces the IngressClass
  plus controller class matching.
- One Gateway maps to one Cloudflare tunnel plus one managed
  cloudflared deployment. Today the tunnel is a single global one fixed
  by a startup flag; Gateway API makes it a per Gateway resource.
- HTTPRoute rules replace Ingress rules. The route to Gateway binding
  through `parentRefs` replaces the ingress class annotation.
- The tunnel domain moves from every Ingress
  `status.loadBalancer` entry to `Gateway.status.addresses`, where it
  belongs.
- Warning events (TLSIgnored, RuleSkipped) become structured status
  conditions (Accepted, ResolvedRefs, Programmed).

The data plane is where it breaks. A Cloudflare tunnel ingress rule can
match only hostname and path. The core of the Gateway API HTTP profile
also requires method matching, header matching, query parameter
matching, and the core filters RequestRedirect and
RequestHeaderModifier. Remote tunnel configuration cannot express any
of that, so a pure cloudflared implementation can never pass the HTTP
conformance profile. This is not a few edge cases to skip; a whole
layer of routing capability is missing.

There is prior art: pl4nty/cloudflare-kubernetes-gateway implements
Gateway API on the Cloudflare API. Reviewing it showed it never sets
`Gateway.status.addresses`, runs the conformance suite in CI with
`continue-on-error`, and has no report merged in the upstream
`conformance/reports` directory. It is useful as wiring reference, not
as proof that the pure tunnel route can conform.

Two more facts shaped the decision:

1. TLS terminates at the Cloudflare edge. Listener certificates have no
   equivalent in the cluster, and TLS passthrough is impossible.
2. Of the 137 conformance test files, only 19 send a request with an
   explicit foreign `Host` header (domains like example.com that cannot
   exist in our zone). Every other test targets the Gateway address
   itself, so it can run across the real edge.

## Decision

We build Gateway API support with an in cluster data plane behind the
tunnel, and we drive the implementation test first with the official
conformance suite running through the real Cloudflare edge.

1. One Gateway provisions one tunnel, one cloudflared deployment, and
   one data plane proxy deployment. The tunnel carries a catch all rule
   that forwards everything to the proxy. The proxy implements the
   HTTPRoute semantics: hostname, path, method, header and query
   matching, and the core filters. cloudflared becomes pure transport.
2. The existing Ingress path stays as it is, still programming tunnel
   ingress rules directly. The two paths share tunnel and DNS plumbing
   but not the routing model.
3. Listener semantics: HTTP and HTTPS listeners are both accepted, and
   both are served by the edge. `certificateRefs` are ignored and the
   listener condition explains why (certificates are managed by
   Cloudflare). TLS mode Passthrough is rejected with `Accepted:
   False`.
4. Every Gateway gets a routable hostname in a real zone (configured
   through GatewayClass `parametersRef`) published in
   `status.addresses`, as a proxied DNS record pointing at the tunnel.
   Per route hostnames keep getting CNAME and ownership TXT records
   exactly like Ingress hostnames do today.
5. The conformance suite is the primary test suite, run end to end:
   test runner to edge to tunnel to cloudflared to proxy to backend.
   It reuses the existing e2e harness (`.env.e2e`, minikube, helm
   install, the dedicated test zone). The red green loop is `go test
   ... --run-test=<TestName>` for one conformance test at a time.
6. Tests that require a foreign `Host` header are skipped with a
   documented reason: the Cloudflare edge routes by Host, so a domain
   outside the zone physically cannot reach the tunnel. This is a
   property of the edge as listener architecture, not an implementation
   gap.

## Alternatives considered

1. Pure cloudflared routing, no proxy. Rejected: hostname plus path is
   the ceiling of tunnel configuration, so core HTTP profile semantics
   are unreachable and the conformance suite degrades into a control
   plane test suite only.
2. Run the conformance suite against the proxy service address inside
   the cluster. Rejected as the primary mode: it skips the edge, the
   tunnel and DNS, which is exactly the production path. The real edge
   run covers all but the foreign Host tests anyway. A cluster local
   run may return later as a fast supplement.
3. A round tripper that rewrites foreign hostnames into zone
   subdomains, to unlock the skipped tests. Rejected: the controller
   and proxy would need a matching rewrite layer, so the suite would be
   testing a distorted system.
4. Adopt or fork pl4nty/cloudflare-kubernetes-gateway. Rejected: it
   shares the pure tunnel routing ceiling, does not manage Gateway
   addresses, and its lifecycle model does not match this controller's
   tunnel and DNS ownership design.

## Consequences

- A new component appears: the data plane proxy, its image, and its
  per Gateway deployment next to cloudflared. How the controller ships
  route configuration to the proxy is specified in
  `docs/design/gateway-api.md`.
- Full conformance including the foreign Host tests is only reachable
  in a cluster local run; the official report from the edge run will
  carry documented skips.
- CI needs the real Cloudflare account secrets to run conformance, the
  same ones the e2e suite already uses.
- The `sigs.k8s.io/gateway-api` module version must stay pinned to the
  installed CRD bundle version, or the suite refuses to run.
- Frequent route changes during the suite hit the Cloudflare API often;
  batching of tunnel configuration updates stays mandatory.
- The CloudflareAccess CRD (ADR 0001) later extends `targetRefs` to
  `kind: HTTPRoute` unchanged, as planned.
