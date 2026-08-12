# Gateway API support: concept mapping, data plane, and conformance driven TDD

Status: draft, companion to ADR 0002.

Tracking issue: #232.

## Goal

Implement the Gateway API HTTP profile (GatewayClass, Gateway,
HTTPRoute) next to the existing Ingress support, test first against the
official conformance suite, with traffic flowing through the real
Cloudflare edge.

## Concept mapping

### Core resources

| Today | Gateway API | Notes |
|---|---|---|
| IngressClass + `--controller-class` flag | GatewayClass `spec.controllerName` | Same matching logic, one field instead of two objects |
| Tunnel fixed by `--cloudflare-tunnel-name` | One Gateway = one tunnel | Tunnel becomes a per Gateway resource with its own lifecycle |
| ControlledCloudflaredConnector (global) | Per Gateway cloudflared deployment | Owned by the Gateway, garbage collected with it |
| (new) | Per Gateway data plane proxy deployment | Implements HTTPRoute semantics, see below |
| Ingress rule (host + paths) | HTTPRoute (`hostnames`, `rules`) | |
| `kubernetes.io/ingress.class` annotation | HTTPRoute `parentRefs` | Binding direction reverses: the route names its Gateway |
| Ingress `status.loadBalancer.ingress[].hostname` | `Gateway.status.addresses` (Hostname type) | The address is a per Gateway zone hostname, not the shared tunnel domain |
| Warning events (TLSIgnored, RuleSkipped, TransformFailed) | Status conditions: Accepted, ResolvedRefs, Programmed | Events stay as a secondary channel |
| Finalizer on Ingress | Finalizer on Gateway and HTTPRoute | Same mechanism |

### Annotations

| Annotation today | Gateway API home |
|---|---|
| `http-host-header` | HTTPRoute URLRewrite filter (`filters.urlRewrite.hostname`) |
| `origin-server-name`, `proxy-ssl-verify`, `no-tls-verify` | BackendTLSPolicy (`validation.hostname` is the SNI) |
| `backend-protocol` | Presence of BackendTLSPolicy, or Service `appProtocol` |
| originRequest family (timeouts, keepalive, http2 origin) | New policy attachment CRD targeting HTTPRoute, same pattern as CloudflareAccess (ADR 0001) |
| `disable-dns-management` | Annotation on the Gateway or route, or folded into the policy CRD |

### Listener semantics

- Protocol HTTP and HTTPS are both accepted. The edge serves both for
  every exposed hostname; listeners only declare intent.
- `certificateRefs` on HTTPS listeners are not required and are
  ignored when present. The listener condition explains that
  certificates live at the Cloudflare edge. This is the structured
  replacement of today's TLSIgnored event for `Ingress.spec.tls`.
- TLS mode Passthrough gets `Accepted: False`. The edge must terminate
  TLS, passthrough is impossible.

### Path types

cloudflared path matching is regular expression based, so Gateway API
path matching maps completely: Exact becomes an anchored expression,
PathPrefix and RegularExpression map directly. This is wider than the
Ingress path support today (Prefix and ImplementationSpecific only).
With the data plane proxy, path matching moves into the proxy anyway
and the tunnel rule stays a catch all.

### New semantics without an Ingress equivalent

- Cross namespace backend references require a ReferenceGrant check.
- Gateway `allowedRoutes` (namespaces, kinds) gates route attachment.
- `status.listeners[].attachedRoutes` must be counted and reported.
- Route level status: every HTTPRoute reports per parent conditions.

## Architecture

```
client
  -> Cloudflare edge          (TLS termination, routes by Host)
  -> tunnel                   (one per Gateway)
  -> cloudflared deployment   (managed, catch all rule)
  -> data plane proxy         (managed, implements HTTPRoute semantics)
  -> backend Service / Pod
```

The tunnel configuration for a Gateway contains a single catch all rule
pointing at the proxy Service. All routing intelligence lives in the
proxy:

- hostname matching (exact and wildcard listeners and route hostnames)
- path matching (Exact, PathPrefix, RegularExpression)
- method, header and query parameter matching
- core filters: RequestRedirect, RequestHeaderModifier
- extended filters as we adopt them: ResponseHeaderModifier,
  URLRewrite, RequestMirror
- backend selection including weights

The proxy is a small Go reverse proxy. The controller compiles the
accepted routes of a Gateway into a routing table and publishes it to
the proxy. The transport for that table (ConfigMap reload, a tiny gRPC
push, or the proxy reading a CRD) is an open implementation question;
whatever is chosen, the proxy must apply updates without dropping
connections, because conformance tests modify routes constantly.

DNS stays as today: every route hostname gets a proxied CNAME to the
tunnel domain plus the `_ctic_managed` ownership TXT record. The
Gateway itself additionally gets one address hostname (below).

### Gateway addressing

The conformance suite reads the address from `Gateway.status.addresses`
and sends every request to it. The shared tunnel domain
(`<tunnel-id>.cfargotunnel.com`) is not reliably reachable directly, so
each Gateway needs a hostname inside a real zone:

- GatewayClass `parametersRef` points at a config object carrying
  `baseDomain` (for e2e: `strrl.cloud` from `.env.e2e`).
- The controller provisions `<gateway-name>-<namespace>.<baseDomain>`
  as a proxied record to the tunnel and publishes it in
  `status.addresses`.
- Requests whose Host equals the address hostname match listeners and
  routes without explicit hostnames, which is exactly what most
  conformance tests rely on.

## Conformance driven TDD

### Harness

Reuse the e2e harness: `.env.e2e` (`CLOUDFLARE_API_TOKEN`,
`CLOUDFLARE_ACCOUNT_ID`, `E2E_BASE_DOMAIN`), a minikube (or kind)
profile, helm install of the controller. On top of that:

- Install the Gateway API CRDs, standard channel, version identical to
  the pinned `sigs.k8s.io/gateway-api` module version.
- A `test/conformance` package calling
  `conformance.RunConformanceWithOptions` with profile `GATEWAY-HTTP`,
  modeled on the invocation in pl4nty/cloudflare-kubernetes-gateway.
- `--timeout-config-overrides` raised generously: every assertion
  crosses the public internet and waits for DNS records, tunnel
  configuration and connector pods to converge.
- The zone must not rewrite traffic: Always Use HTTPS and automatic
  HTTPS rewrites stay off for the test zone, otherwise the suite sees
  redirects it did not ask for.

### The loop

The suite setup is the first red test: base manifests wait for the
GatewayClass to report `Accepted: True`. From there, one conformance
test at a time:

```
go test ./test/conformance -args \
  --gateway-class=cloudflare-tunnel \
  --run-test=HTTPRouteSimpleSameNamespace \
  --debug
```

Suggested order, each step a red green cycle:

1. GatewayClass acceptance (suite setup gate), then
   `GatewayClassObservedGenerationBump`.
2. Gateway control plane: invalid listener rejection, observed
   generation, `GatewayWithAttachedRoutes`, listener conditions.
3. Gateway provisioning: tunnel, cloudflared, proxy, address in
   `status.addresses`. First moment real infrastructure exists.
4. HTTPRoute control plane: the `httproute-invalid-*` family,
   ReferenceGrant tests, cross namespace rules.
5. Data plane basics through the edge:
   `HTTPRouteSimpleSameNamespace`, exact and prefix path matching.
6. Matching depth: header, method, query parameter tests.
7. Filters: redirects, header modifiers, then extended features.

Full profile runs (with the skip list) happen in CI with
`--report-output`, uploading the report as an artifact.

### Skip list for the edge run

These tests send a `Host` header for domains that cannot exist in the
test zone. The edge routes by Host, so the requests never reach the
tunnel. They are skipped in the edge run with this reason recorded;
they can only pass in a cluster local run against the proxy Service,
which may be added later as a supplement.

Candidates found by scanning the suite (verify per test during
implementation):

- `GatewayHTTPListenerIsolation`
- `HTTPRouteHostnameIntersection`
- `HTTPRouteListenerHostnameMatching`
- `HTTPRouteListenerPortMatching`
- `HTTPRouteMatchingAcrossRoutes`
- `HTTPRouteRedirectHostAndStatus`
- `HTTPRouteRedirectPath`
- `HTTPRouteRedirectPort`
- `HTTPRouteRewriteHost`

## Open questions

1. Route table transport between controller and proxy (ConfigMap
   reload vs push vs proxy side informers).
2. Whether the proxy and cloudflared run as two deployments or two
   containers in one pod. Two containers remove a Service hop but
   couple their lifecycles.
3. Exact `parametersRef` schema (ConfigMap vs a small CRD) and what
   else belongs in it (proxy image, replica counts, resources).
4. Whether direct requests to `<tunnel-id>.cfargotunnel.com` work at
   all today; if they do, the address hostname could become optional.
5. Rate limit behavior of the Cloudflare API under conformance churn;
   the suite creates and deletes routes far faster than human users.
