# Gateway API support: concept mapping, data plane, and conformance driven TDD

Status: proof of concept implemented, companion to ADR 0002.

Tracking issue: #232.

## Goal

Implement the Gateway API HTTP profile (GatewayClass, Gateway,
HTTPRoute) next to the existing Ingress support, test first against the
official conformance suite, with traffic flowing through the real
Cloudflare edge.

## Result

Gateway API `v1.6.2`, profile `GATEWAY-HTTP`, core features Gateway,
HTTPRoute and ReferenceGrant: 37 core tests, 33 pass through the real
edge, 4 are skipped (see the skip list), 0 fail. Two full runs in a row
pass, each takes about one minute.

Measured on a local minikube:

- a new Gateway is Programmed in about 3s and answers through the edge
  in about 6s
- a route change reaches the proxy in about 1s and costs no Cloudflare
  API call
- one full run makes about 120 to 160 Cloudflare API calls, far below
  the limit of 1200 per 5 minutes

## Concept mapping

### Core resources

| Today | Gateway API | Notes |
|---|---|---|
| IngressClass + `--controller-class` flag | GatewayClass `spec.controllerName` | The GatewayClass reuses the value of `--controller-class` |
| Tunnel fixed by `--cloudflare-tunnel-name` | Shared tunnel | The PoC keeps one tunnel for Ingress and every Gateway, see PoC deviations |
| ControlledCloudflaredConnector (global) | The same connector | Carries the traffic of every Gateway |
| (new) | Per Gateway data plane proxy deployment | Implements HTTPRoute semantics, see below |
| Ingress rule (host + paths) | HTTPRoute (`hostnames`, `rules`) | |
| `kubernetes.io/ingress.class` annotation | HTTPRoute `parentRefs` | Binding direction reverses: the route names its Gateway |
| Ingress `status.loadBalancer.ingress[].hostname` | `Gateway.status.addresses` (Hostname type) | The address is a per Gateway zone hostname, not the shared tunnel domain |
| Warning events (TLSIgnored, RuleSkipped, TransformFailed) | Status conditions: Accepted, ResolvedRefs, Programmed | |
| Finalizer on Ingress | No finalizer | Every reconcile recomputes the full state, stale proxies and DNS records are removed when their Gateway is gone |

### Annotations

| Annotation today | Gateway API home |
|---|---|
| `http-host-header` | HTTPRoute URLRewrite filter (`filters.urlRewrite.hostname`) |
| `origin-server-name`, `proxy-ssl-verify`, `no-tls-verify` | BackendTLSPolicy (`validation.hostname` is the SNI) |
| `backend-protocol` | Presence of BackendTLSPolicy, or Service `appProtocol` |
| originRequest family (timeouts, keepalive, http2 origin) | New policy attachment CRD targeting HTTPRoute, same pattern as CloudflareAccess (ADR 0001) |
| `disable-dns-management` | Annotation on the Gateway or route, or folded into the policy CRD |

None of these are implemented in the PoC.

### Listener semantics

- Protocol HTTP and HTTPS are accepted, every other protocol gets
  `Accepted: False` with reason UnsupportedProtocol. The edge serves
  every Gateway address over both schemes, a zone with Always Use HTTPS
  only over https. The proxy therefore does not separate requests by
  scheme, the listener protocol only decides the default scheme of
  redirects.
- `certificateRefs` are validated but never used: wrong group or kind,
  a missing Secret or an unparsable certificate give `ResolvedRefs:
  False` with reason InvalidCertificateRef, a cross namespace reference
  without ReferenceGrant gives RefNotPermitted. The certificate the
  client sees is the Cloudflare edge certificate.
- TLS mode Passthrough gets `Accepted: False`, the edge must terminate
  TLS.
- A listener with unresolved references is `Programmed: False`, the
  Gateway as a whole is still Programmed when at least one listener is
  accepted.
- `spec.infrastructure.parametersRef` on a Gateway is not supported and
  gives `Accepted: False` with reason InvalidParameters.

### Path types

Path matching lives in the proxy: Exact, PathPrefix (on whole path
segments, a trailing slash in the prefix is ignored) and
RegularExpression. Header and query parameter matches support Exact and
RegularExpression.

### New semantics without an Ingress equivalent

- Cross namespace backend references require a ReferenceGrant check.
- Gateway `allowedRoutes` (namespaces, kinds) gates route attachment.
- `status.listeners[].attachedRoutes` counts every accepted route once
  per listener.
- Route level status: every HTTPRoute reports Accepted and ResolvedRefs
  per parent. Entries of other controllers are kept untouched.

## Architecture

```
client
  -> Cloudflare edge          (TLS termination, routes by Host)
  -> tunnel                   (shared, one rule per Gateway address)
  -> cloudflared deployment   (the existing managed connector)
  -> data plane proxy         (one per Gateway, implements HTTPRoute semantics)
  -> backend Service / Pod
```

The tunnel configuration has one rule per Gateway: the Gateway address
points at the ClusterIP Service of its proxy. All routing intelligence
lives in the proxy:

- hostname matching: the most specific matching hostname (exact, then
  wildcards with more labels, then any host) is chosen first, then only
  its routes are considered
- path, method, header and query parameter matching
- core filters RequestRedirect and RequestHeaderModifier, a rule with an
  unsupported filter answers 500
- weighted backend selection, invalid backends answer 500

### Controller

One reconcile loop handles every Gateway and HTTPRoute. Any watched
change (Gateway, HTTPRoute, GatewayClass, ReferenceGrant, Namespace
labels, Service, EndpointSlice, parameter ConfigMap, proxy Deployment)
enqueues the same key, so bursts collapse into one run and route status,
which depends on several Gateways, and Gateway status, which depends on
several routes, are always computed from one consistent snapshot. A run:

1. validates the GatewayClasses and their parameters
2. validates every Gateway and its listeners
3. attaches every route to its parents: sectionName and port, listener
   kinds and namespaces, hostname intersection
4. compiles one routing table per accepted Gateway, sorted by Gateway
   API precedence, and writes it with the proxy Deployment and Service
5. pushes the Gateway exposures to Cloudflare, skipped when the set did
   not change since the last successful push
6. writes status: `Programmed: True` and the address only after the
   Cloudflare push succeeded and the proxy is ready
7. removes proxy resources whose Gateway is gone

The GatewayClass has its own small controller that sets `Accepted`.

Headless Services have no cluster IP: the controller resolves them
through their EndpointSlices and writes the endpoint addresses into the
routing table.

### Route table transport

The controller writes the compiled table as JSON into the ConfigMap
`gateway-<hash of namespace/name>` in the controller namespace. The
proxy watches exactly that object through the API and swaps the table
atomically, requests in flight keep the table they started with. A
mounted volume was not used, its update delay of up to a minute is too
slow for the suite.

The proxy is the `proxy` subcommand of the controller binary, so it
ships in the same image. Its Deployment, Service and ConfigMap carry the
label `strrl.dev/gateway-proxy` and the controller Deployment as owner,
so uninstalling the controller removes them.

### Tunnel sync

`PutExposures` replaces the whole tunnel configuration and removes DNS
records it does not see, so Ingress and Gateway exposures are always
pushed together under one lock. The Ingress side keeps pushing on every
reconcile like before. The Gateway side remembers the last pushed set and
only calls Cloudflare when it changed, so route churn is free. DNS
records whose content already matches are no longer updated, and every
call is counted in `cloudflare_tunnel_ingress_controller_cloudflare_api_requests_total{operation}`.

### Gateway addressing

The conformance suite reads the address from `Gateway.status.addresses`
and sends every request to it. The shared tunnel domain
(`<tunnel-id>.cfargotunnel.com`) is not used for that, every Gateway
gets a hostname inside a real zone instead, which also carries the edge
certificate.

- GatewayClass `parametersRef` points at a ConfigMap in the controller
  namespace (created by the helm chart) with the keys `baseDomain`
  (required) and `labelSuffix` (optional). A missing ref, a ConfigMap in
  another namespace, a missing ConfigMap or an empty `baseDomain` give
  `Accepted: False` with reason InvalidParameters.
- The address is `<name>-<namespace><labelSuffix>.<baseDomain>`. A first
  label longer than 63 characters is cut, gets 8 hex characters of
  sha256(`namespace/name`) and keeps the suffix.
- With `baseDomain` set to the zone apex the address is one level below
  the zone, so the Universal SSL certificate covers it and https works.
  A deeper `baseDomain` needs Advanced Certificate Manager for https.
- The address is a proxied CNAME to the tunnel plus the `_ctic_managed`
  ownership TXT record, written by the existing DNS code.
- Requests whose Host equals the address match listeners and routes
  without explicit hostnames, which is what most conformance tests rely
  on.

Per route hostnames get no DNS records in the PoC: every route hostname
in the core tests is outside the zone.

## Conformance driven TDD

### Harness

```
make conformance-up                              # cleanup, minikube, CRDs, deploy
make conformance-deploy                          # rebuild image, helm upgrade
make conformance RUN_TEST=HTTPRouteSimpleSameNamespace
make conformance                                 # full run, writes test/conformance/artifacts/report.yaml
make conformance-down                            # namespaces, release, Cloudflare leftovers, cluster
make conformance-clean                           # Cloudflare leftovers only, no cluster needed
```

- Credentials come from `ENV_FILE` (default `./.env.e2e`), loaded with
  `set -a` and never printed.
- `RUN_ID` (default `local-$(whoami)`, CI uses `ci`) separates
  environments: minikube profile `ctic-gwc-<RUN_ID>`, tunnel
  `<CLOUDFLARE_TUNNEL_NAME>-gwc-<RUN_ID>`, address suffix
  `-gwc-<RUN_ID>`. The cleanup tool deletes every record ending in
  `-gwc-<RUN_ID>.<E2E_BASE_DOMAIN>` and resets the tunnel to the single
  `http_status:404` rule, also after a crashed run.
- A single test run keeps the base Gateways (`--cleanup-base-resources=false`),
  a rerun takes about 5 seconds. A full run cleans them up.
- The runner resolves Gateway addresses with the zone's authoritative
  nameservers (`CONFORMANCE_RESOLVER` overrides), so no local resolver
  caches a negative answer from before the record existed.
- The runner sends the suite's plain http requests to the edge over
  https. The test zone answers http with its own redirect (Always Use
  HTTPS), so https is what every client ends up with. Host, path,
  headers and the rest of the request stay as the suite sent them, and
  the request still travels edge, tunnel, cloudflared, proxy, backend.
- Timeouts: `MaxTimeToConsistency:90;GatewayMustHaveAddress:240;GatewayMustHaveCondition:240`.

### Skip list for the edge run

Verified against `conformance/v1.6.2`. These tests send a `Host` header
for domains that cannot exist in the test zone, or check the served
certificate. The edge routes by Host and presents its own certificate,
so the requests never reach the tunnel.

| Skipped test | Reason |
|---|---|
| HTTPRouteHTTPSListener | TLS with SNI and Host `example.org` variants, checks the served certificate against the test's own Secret |
| HTTPRouteHostnameIntersection | Hosts `very.specific.com`, `foo.wildcard.io`, `first.com` and others are outside the zone |
| HTTPRouteListenerHostnameMatching | Hosts `bar.com`, `foo.bar.com`, `foo.com` are outside the zone |
| HTTPRouteMatchingAcrossRoutes | Hosts `example.com` and `example.net` are outside the zone |

The logic behind them is implemented: hostname intersection drives
attachment, `attachedRoutes` and the NoMatchingListenerHostname reason.
Their request cases run as unit tests in `pkg/controller`: the
controller compiles the test manifests, the real proxy serves the table
and real HTTP backends answer.

HTTPRouteRedirectHostAndStatus is not skipped: `example.org` only
appears in the expected Location header.

## PoC deviations

- Shared tunnel: one tunnel for the Ingress rules and every Gateway, the
  existing cloudflared connector carries all traffic. One tunnel and one
  connector per Gateway stays the target after the PoC; for about 20
  Gateways per run it would add tunnel creation, token fetch, a
  connector Deployment and 10 to 20 seconds of connector registration
  each, and leak tunnels after crashes.
- `certificateRefs` are validated but unused, the edge certificate is
  served.
- A `baseDomain` below the zone apex needs Advanced Certificate Manager
  for https.
- Per route hostnames get no DNS records.
- The runner talks https to the edge, see Harness.

## Resolved questions

1. Route table transport: a ConfigMap per Gateway, watched by the proxy
   through the API.
2. Proxy and cloudflared: separate Deployments. cloudflared is the shared
   connector, the proxy is per Gateway.
3. `parametersRef` schema: a ConfigMap in the controller namespace with
   `baseDomain` and the optional `labelSuffix`. Proxy image and
   resources come from controller flags.
4. Direct requests to `<tunnel-id>.cfargotunnel.com` are not used, every
   Gateway needs a zone hostname anyway for the edge certificate.
5. Rate limits: the skip when unchanged push keeps a full run at about
   120 to 160 calls and route churn at zero.
