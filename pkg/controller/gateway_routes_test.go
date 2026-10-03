package controller

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/gatewayproxy"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/yaml"
)

func TestGatewayAddressLabel(t *testing.T) {
	assert.Equal(t, "same-namespace-gateway-conformance-infra", gatewayAddressLabel("gateway-conformance-infra", "same-namespace", ""))
	assert.Equal(t, "same-namespace-gateway-conformance-infra-gwc-ci", gatewayAddressLabel("gateway-conformance-infra", "same-namespace", "-gwc-ci"))

	long := gatewayAddressLabel("gateway-conformance-infra", "gateway-secret-reference-grant-all-in-namespace", "-gwc-local-strrl")
	assert.LessOrEqual(t, len(long), 63)
	assert.True(t, strings.HasPrefix(long, "gateway-secret-reference-grant-all-in-"), long)
	assert.True(t, strings.HasSuffix(long, "-gwc-local-strrl"), long)
	assert.Equal(t, long, gatewayAddressLabel("gateway-conformance-infra", "gateway-secret-reference-grant-all-in-namespace", "-gwc-local-strrl"), "the label must be stable")
	assert.NotEqual(t, long, gatewayAddressLabel("gateway-conformance-infra", "gateway-secret-reference-grant-all-in-namespace-2", "-gwc-local-strrl"))

	// a cut that ends with a dash does not produce a double dash
	cut := gatewayAddressLabel("ns", strings.Repeat("a", 53)+"-bbbbbbbbbbbbbbbb", "")
	assert.NotContains(t, cut, "--")
	assert.LessOrEqual(t, len(cut), 63)
}

func TestHostnameIntersection(t *testing.T) {
	cases := []struct {
		listener string
		route    string
		result   string
		ok       bool
	}{
		{listener: "", route: "", result: "", ok: true},
		{listener: "", route: "foo.com", result: "foo.com", ok: true},
		{listener: "foo.com", route: "", result: "foo.com", ok: true},
		{listener: "foo.com", route: "foo.com", result: "foo.com", ok: true},
		{listener: "foo.com", route: "bar.com", ok: false},
		{listener: "*.wildcard.io", route: "foo.wildcard.io", result: "foo.wildcard.io", ok: true},
		{listener: "*.wildcard.io", route: "foo.bar.wildcard.io", result: "foo.bar.wildcard.io", ok: true},
		{listener: "*.wildcard.io", route: "wildcard.io", ok: false},
		{listener: "very.specific.com", route: "*.specific.com", result: "very.specific.com", ok: true},
		{listener: "specific.com", route: "*.specific.com", ok: false},
		{listener: "*.anotherwildcard.io", route: "*.anotherwildcard.io", result: "*.anotherwildcard.io", ok: true},
		{listener: "*.example.com", route: "*.foo.example.com", result: "*.foo.example.com", ok: true},
		{listener: "*.foo.example.com", route: "*.example.com", result: "*.foo.example.com", ok: true},
		{listener: "*.example.com", route: "*.example.net", ok: false},
	}
	for _, tc := range cases {
		result, ok := hostnameIntersection(tc.listener, tc.route)
		assert.Equal(t, tc.ok, ok, "%q and %q", tc.listener, tc.route)
		if tc.ok {
			assert.Equal(t, tc.result, result, "%q and %q", tc.listener, tc.route)
		}
	}
}

func TestReferenceGrantAllows(t *testing.T) {
	grants := []gatewayv1.ReferenceGrant{{
		ObjectMeta: metav1.ObjectMeta{Namespace: "backend", Name: "grant"},
		Spec: gatewayv1.ReferenceGrantSpec{
			From: []gatewayv1.ReferenceGrantFrom{{Group: gatewayGroup, Kind: kindHTTPRte, Namespace: "infra"}},
			To:   []gatewayv1.ReferenceGrantTo{{Group: "", Kind: kindService, Name: new(gatewayv1.ObjectName("web"))}},
		},
	}}
	assert.True(t, referenceGrantAllows(grants, gatewayGroup, kindHTTPRte, "infra", "", kindService, "backend", "web"))
	assert.False(t, referenceGrantAllows(grants, gatewayGroup, kindHTTPRte, "infra", "", kindService, "backend", "other"), "wrong name")
	assert.False(t, referenceGrantAllows(grants, gatewayGroup, kindGateway, "infra", "", kindService, "backend", "web"), "wrong from kind")
	assert.False(t, referenceGrantAllows(grants, gatewayGroup, kindHTTPRte, "other", "", kindService, "backend", "web"), "wrong from namespace")
	assert.False(t, referenceGrantAllows(grants, gatewayGroup, kindHTTPRte, "infra", "", kindSecret, "backend", "web"), "wrong to kind")
	assert.False(t, referenceGrantAllows(grants, gatewayGroup, kindHTTPRte, "infra", "", kindService, "infra", "web"), "grant must live in the target namespace")
}

// scenario is a set of Gateways and routes compiled and served by a real
// proxy, with real HTTP backends behind headless services.
type scenario struct {
	t        *testing.T
	snapshot *clusterSnapshot
	states   map[types.NamespacedName]*gatewayState
	results  map[types.NamespacedName][]parentResult
}

const testNamespace = "gateway-conformance-infra"

func newScenario(t *testing.T, manifests string) *scenario {
	t.Helper()
	snapshot := &clusterSnapshot{
		namespaces: map[string]*corev1.Namespace{
			testNamespace: {ObjectMeta: metav1.ObjectMeta{Name: testNamespace, Labels: map[string]string{"kubernetes.io/metadata.name": testNamespace}}},
		},
		services:       map[types.NamespacedName]*corev1.Service{},
		endpointSlices: map[types.NamespacedName][]discoveryv1.EndpointSlice{},
		clusterDomain:  "cluster.local",
	}
	for _, name := range []string{"infra-backend-v1", "infra-backend-v2", "infra-backend-v3"} {
		addBackend(t, snapshot, name)
	}

	var gateways []*gatewayv1.Gateway
	var routes []gatewayv1.HTTPRoute
	for index, document := range strings.Split(manifests, "\n---\n") {
		var meta metav1.TypeMeta
		require.NoError(t, yaml.Unmarshal([]byte(document), &meta))
		switch meta.Kind {
		case "Gateway":
			gateway := &gatewayv1.Gateway{}
			require.NoError(t, yaml.Unmarshal([]byte(document), gateway))
			gateways = append(gateways, gateway)
		case "HTTPRoute":
			route := gatewayv1.HTTPRoute{}
			require.NoError(t, yaml.Unmarshal([]byte(document), &route))
			// older routes win ties, keep the manifest order
			route.CreationTimestamp = metav1.NewTime(time.Unix(int64(1000+index), 0))
			routes = append(routes, route)
		}
	}

	states := map[types.NamespacedName]*gatewayState{}
	for _, gateway := range gateways {
		state, err := buildGatewayState(context.Background(), gateway, gatewayAddressing{baseDomain: "example.net"}, snapshot, nil)
		require.NoError(t, err)
		states[types.NamespacedName{Namespace: gateway.Namespace, Name: gateway.Name}] = state
	}
	results := attachRoutes(states, routes, snapshot)
	return &scenario{t: t, snapshot: snapshot, states: states, results: results}
}

// addBackend starts a real HTTP server and exposes it as a headless service
// whose endpoint slice points at it.
func addBackend(t *testing.T, snapshot *clusterSnapshot, name string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"backend": name})
	}))
	t.Cleanup(server.Close)
	host, portText, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)

	key := types.NamespacedName{Namespace: testNamespace, Name: name}
	snapshot.services[key] = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: name},
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Ports:     []corev1.ServicePort{{Name: "first-port", Port: 8080}},
		},
	}
	snapshot.endpointSlices[key] = []discoveryv1.EndpointSlice{{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: name + "-abc"},
		Endpoints:  []discoveryv1.Endpoint{{Addresses: []string{host}}},
		Ports:      []discoveryv1.EndpointPort{{Name: new("first-port"), Port: new(int32(port))}},
	}}
}

// serve compiles the table of a Gateway and serves it with the real proxy.
func (s *scenario) serve(gatewayName string) string {
	s.t.Helper()
	state := s.states[types.NamespacedName{Namespace: testNamespace, Name: gatewayName}]
	require.NotNil(s.t, state)
	table := compileTable(state, s.snapshot)
	server := gatewayproxy.NewServer(logr.Discard())
	require.NoError(s.t, server.SetTable(&table))
	proxy := httptest.NewServer(server)
	s.t.Cleanup(proxy.Close)
	return proxy.URL
}

// request sends a request with the given Host and returns the status and the
// name of the backend that answered.
func request(t *testing.T, proxyURL string, host string, path string, headers map[string]string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, proxyURL+path, nil)
	require.NoError(t, err)
	req.Host = host
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	response, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return response.StatusCode, ""
	}
	body := map[string]string{}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
	return response.StatusCode, body["backend"]
}

type requestCase struct {
	host    string
	path    string
	headers map[string]string
	status  int
	backend string
}

func (s *scenario) check(proxyURL string, cases []requestCase) {
	s.t.Helper()
	for _, tc := range cases {
		status, backend := request(s.t, proxyURL, tc.host, tc.path, tc.headers)
		expectedStatus := tc.status
		if expectedStatus == 0 {
			expectedStatus = http.StatusOK
		}
		assert.Equal(s.t, expectedStatus, status, "%s%s %v", tc.host, tc.path, tc.headers)
		if expectedStatus == http.StatusOK {
			assert.Equal(s.t, tc.backend, backend, "%s%s %v", tc.host, tc.path, tc.headers)
		}
	}
}

func (s *scenario) attachedRoutes(gatewayName string) map[string]int32 {
	result := map[string]int32{}
	for _, item := range s.states[types.NamespacedName{Namespace: testNamespace, Name: gatewayName}].listeners {
		result[string(item.listener.Name)] = item.attachedRoutes
	}
	return result
}

func (s *scenario) parentReason(routeName string) gatewayv1.RouteConditionReason {
	results := s.results[types.NamespacedName{Namespace: testNamespace, Name: routeName}]
	require.Len(s.t, results, 1)
	return results[0].reason
}

// The request cases of the conformance test HTTPRouteHostnameIntersection,
// which is skipped through the edge because its hosts are outside the zone.
func TestHostnameIntersectionConformanceCases(t *testing.T) {
	s := newScenario(t, `
kind: Gateway
metadata: {name: httproute-hostname-intersection, namespace: gateway-conformance-infra}
spec:
  gatewayClassName: cloudflare-tunnel
  listeners:
  - {name: listener-1, port: 80, protocol: HTTP, allowedRoutes: {namespaces: {from: Same}}, hostname: very.specific.com}
  - {name: listener-2, port: 80, protocol: HTTP, allowedRoutes: {namespaces: {from: Same}}, hostname: "*.wildcard.io"}
  - {name: listener-3, port: 80, protocol: HTTP, allowedRoutes: {namespaces: {from: Same}}, hostname: "*.anotherwildcard.io"}
---
kind: Gateway
metadata: {name: httproute-hostname-intersection-all, namespace: gateway-conformance-infra}
spec:
  gatewayClassName: cloudflare-tunnel
  listeners:
  - {name: listener-1, port: 80, protocol: HTTP, allowedRoutes: {namespaces: {from: Same}}}
---
kind: HTTPRoute
metadata: {name: specific-host-matches-listener-specific-host, namespace: gateway-conformance-infra}
spec:
  parentRefs: [{name: httproute-hostname-intersection, namespace: gateway-conformance-infra}]
  hostnames: [non.matching.com, "*.nonmatchingwildcard.io", very.specific.com]
  rules:
  - matches: [{path: {type: PathPrefix, value: /s1}}]
    backendRefs: [{name: infra-backend-v1, port: 8080}]
---
kind: HTTPRoute
metadata: {name: specific-host-matches-listener-wildcard-host, namespace: gateway-conformance-infra}
spec:
  parentRefs: [{name: httproute-hostname-intersection, namespace: gateway-conformance-infra}]
  hostnames: [non.matching.com, wildcard.io, foo.wildcard.io, bar.wildcard.io, foo.bar.wildcard.io]
  rules:
  - matches: [{path: {type: PathPrefix, value: /s2}}]
    backendRefs: [{name: infra-backend-v2, port: 8080}]
---
kind: HTTPRoute
metadata: {name: wildcard-host-matches-listener-specific-host, namespace: gateway-conformance-infra}
spec:
  parentRefs: [{name: httproute-hostname-intersection}]
  hostnames: [non.matching.com, "*.specific.com"]
  rules:
  - matches: [{path: {type: PathPrefix, value: /s3}}]
    backendRefs: [{name: infra-backend-v3, port: 8080}]
---
kind: HTTPRoute
metadata: {name: wildcard-host-matches-listener-wildcard-host, namespace: gateway-conformance-infra}
spec:
  parentRefs: [{name: httproute-hostname-intersection}]
  hostnames: ["*.anotherwildcard.io"]
  rules:
  - matches: [{path: {type: PathPrefix, value: /s4}}]
    backendRefs: [{name: infra-backend-v1, port: 8080}]
---
kind: HTTPRoute
metadata: {name: no-intersecting-hosts, namespace: gateway-conformance-infra}
spec:
  parentRefs: [{name: httproute-hostname-intersection}]
  hostnames: [specific.but.wrong.com, wildcard.io]
  rules:
  - matches: [{path: {type: PathPrefix, value: /s5}}]
    backendRefs: [{name: infra-backend-v2, port: 8080}]
---
kind: HTTPRoute
metadata: {name: httproute-hostname-intersection-all, namespace: gateway-conformance-infra}
spec:
  parentRefs: [{name: httproute-hostname-intersection-all}]
  hostnames: [first.com, sub.first.com, second.com, sub.second.com]
  rules:
  - backendRefs: [{name: infra-backend-v2, port: 8080}]
`)

	assert.Equal(t, map[string]int32{"listener-1": 2, "listener-2": 1, "listener-3": 1}, s.attachedRoutes("httproute-hostname-intersection"))
	assert.Equal(t, gatewayv1.RouteReasonNoMatchingListenerHostname, s.parentReason("no-intersecting-hosts"))

	proxyURL := s.serve("httproute-hostname-intersection")
	s.check(proxyURL, []requestCase{
		{host: "very.specific.com", path: "/s1", backend: "infra-backend-v1"},
		{host: "very.specific.com:1234", path: "/s1", backend: "infra-backend-v1"},
		{host: "non.matching.com", path: "/s1", status: 404},
		{host: "foo.nonmatchingwildcard.io", path: "/s1", status: 404},
		{host: "foo.wildcard.io", path: "/s1", status: 404},
		{host: "very.specific.com", path: "/non-matching-prefix", status: 404},
		{host: "foo.wildcard.io", path: "/s2", backend: "infra-backend-v2"},
		{host: "bar.wildcard.io", path: "/s2", backend: "infra-backend-v2"},
		{host: "foo.bar.wildcard.io", path: "/s2", backend: "infra-backend-v2"},
		{host: "non.matching.com", path: "/s2", status: 404},
		{host: "wildcard.io", path: "/s2", status: 404},
		{host: "very.specific.com", path: "/s2", status: 404},
		{host: "foo.wildcard.io", path: "/non-matching-prefix", status: 404},
		{host: "very.specific.com", path: "/s3", backend: "infra-backend-v3"},
		{host: "non.matching.com", path: "/s3", status: 404},
		{host: "foo.specific.com", path: "/s3", status: 404},
		{host: "foo.wildcard.io", path: "/s3", status: 404},
		{host: "very.specific.com", path: "/non-matching-prefix", status: 404},
		{host: "foo.anotherwildcard.io", path: "/s4", backend: "infra-backend-v1"},
		{host: "bar.anotherwildcard.io", path: "/s4", backend: "infra-backend-v1"},
		{host: "foo.bar.anotherwildcard.io", path: "/s4", backend: "infra-backend-v1"},
		{host: "anotherwildcard.io", path: "/s4", status: 404},
		{host: "foo.wildcard.io", path: "/s4", status: 404},
		{host: "very.specific.com", path: "/s4", status: 404},
		{host: "foo.anotherwildcard.io", path: "/non-matching-prefix", status: 404},
		{host: "specific.but.wrong.com", path: "/s5", status: 404},
		{host: "wildcard.io", path: "/s5", status: 404},
	})

	s.check(s.serve("httproute-hostname-intersection-all"), []requestCase{
		{host: "first.com", path: "/", backend: "infra-backend-v2"},
		{host: "sub.first.com", path: "/", backend: "infra-backend-v2"},
		{host: "second.com", path: "/", backend: "infra-backend-v2"},
		{host: "sub.second.com", path: "/", backend: "infra-backend-v2"},
		{host: "third.com", path: "/", status: 404},
		{host: "sub.third.com", path: "/", status: 404},
	})
}

// The request cases of the conformance test HTTPRouteListenerHostnameMatching.
func TestListenerHostnameMatchingConformanceCases(t *testing.T) {
	s := newScenario(t, `
kind: Gateway
metadata: {name: httproute-listener-hostname-matching, namespace: gateway-conformance-infra}
spec:
  gatewayClassName: cloudflare-tunnel
  listeners:
  - {name: listener-1, port: 80, protocol: HTTP, allowedRoutes: {namespaces: {from: Same}}, hostname: bar.com}
  - {name: listener-2, port: 80, protocol: HTTP, allowedRoutes: {namespaces: {from: Same}}, hostname: foo.bar.com}
  - {name: listener-3, port: 80, protocol: HTTP, allowedRoutes: {namespaces: {from: Same}}, hostname: "*.bar.com"}
  - {name: listener-4, port: 80, protocol: HTTP, allowedRoutes: {namespaces: {from: Same}}, hostname: "*.foo.com"}
---
kind: HTTPRoute
metadata: {name: backend-v1, namespace: gateway-conformance-infra}
spec:
  parentRefs: [{name: httproute-listener-hostname-matching, namespace: gateway-conformance-infra, sectionName: listener-1}]
  rules: [{backendRefs: [{name: infra-backend-v1, port: 8080}]}]
---
kind: HTTPRoute
metadata: {name: backend-v2, namespace: gateway-conformance-infra}
spec:
  parentRefs: [{name: httproute-listener-hostname-matching, namespace: gateway-conformance-infra, sectionName: listener-2}]
  rules: [{backendRefs: [{name: infra-backend-v2, port: 8080}]}]
---
kind: HTTPRoute
metadata: {name: backend-v3, namespace: gateway-conformance-infra}
spec:
  parentRefs:
  - {name: httproute-listener-hostname-matching, namespace: gateway-conformance-infra, sectionName: listener-3}
  - {name: httproute-listener-hostname-matching, namespace: gateway-conformance-infra, sectionName: listener-4}
  rules: [{backendRefs: [{name: infra-backend-v3, port: 8080}]}]
`)

	s.check(s.serve("httproute-listener-hostname-matching"), []requestCase{
		{host: "bar.com", path: "/", backend: "infra-backend-v1"},
		{host: "foo.bar.com", path: "/", backend: "infra-backend-v2"},
		{host: "baz.bar.com", path: "/", backend: "infra-backend-v3"},
		{host: "boo.bar.com", path: "/", backend: "infra-backend-v3"},
		{host: "multiple.prefixes.bar.com", path: "/", backend: "infra-backend-v3"},
		{host: "multiple.prefixes.foo.com", path: "/", backend: "infra-backend-v3"},
		{host: "foo.com", path: "/", status: 404},
		{host: "no.matching.host", path: "/", status: 404},
	})
}

// The request cases of the conformance test HTTPRouteMatchingAcrossRoutes.
func TestMatchingAcrossRoutesConformanceCases(t *testing.T) {
	s := newScenario(t, `
kind: Gateway
metadata: {name: same-namespace, namespace: gateway-conformance-infra}
spec:
  gatewayClassName: cloudflare-tunnel
  listeners:
  - {name: http, port: 80, protocol: HTTP, allowedRoutes: {namespaces: {from: Same}}}
---
kind: HTTPRoute
metadata: {name: matching-part1, namespace: gateway-conformance-infra}
spec:
  parentRefs: [{name: same-namespace}]
  hostnames: [example.com, example.net]
  rules:
  - matches:
    - path: {type: PathPrefix, value: /}
    - headers: [{name: version, value: one}]
    backendRefs: [{name: infra-backend-v1, port: 8080}]
---
kind: HTTPRoute
metadata: {name: matching-part2, namespace: gateway-conformance-infra}
spec:
  parentRefs: [{name: same-namespace}]
  hostnames: [example.com]
  rules:
  - matches:
    - path: {type: PathPrefix, value: /v2}
    - headers: [{name: version, value: two}]
    backendRefs: [{name: infra-backend-v2, port: 8080}]
`)

	s.check(s.serve("same-namespace"), []requestCase{
		{host: "example.com", path: "/", backend: "infra-backend-v1"},
		{host: "example.com", path: "/example", backend: "infra-backend-v1"},
		{host: "example.net", path: "/example", backend: "infra-backend-v1"},
		{host: "example.com", path: "/example", headers: map[string]string{"Version": "one"}, backend: "infra-backend-v1"},
		{host: "example.com", path: "/v2", backend: "infra-backend-v2"},
		{host: "example.net", path: "/v2", backend: "infra-backend-v1"},
		{host: "example.com", path: "/v2/example", backend: "infra-backend-v2"},
		{host: "example.com", path: "/", headers: map[string]string{"Version": "two"}, backend: "infra-backend-v2"},
	})
}

// The precedence cases of HTTPRoutePathMatchOrder, compiled by the
// controller instead of a hand sorted table.
func TestPathMatchOrderPrecedence(t *testing.T) {
	s := newScenario(t, `
kind: Gateway
metadata: {name: same-namespace, namespace: gateway-conformance-infra}
spec:
  gatewayClassName: cloudflare-tunnel
  listeners:
  - {name: http, port: 80, protocol: HTTP}
---
kind: HTTPRoute
metadata: {name: path-matching-order, namespace: gateway-conformance-infra}
spec:
  parentRefs: [{name: same-namespace}]
  rules:
  - {matches: [{path: {type: Exact, value: /match}}], backendRefs: [{name: infra-backend-v1, port: 8080}]}
  - {matches: [{path: {type: Exact, value: /match/exact}}], backendRefs: [{name: infra-backend-v2, port: 8080}]}
  - {matches: [{path: {type: Exact, value: /match/exact/one}}], backendRefs: [{name: infra-backend-v3, port: 8080}]}
  - {matches: [{path: {type: PathPrefix, value: /match/}}], backendRefs: [{name: infra-backend-v3, port: 8080}]}
  - {matches: [{path: {type: PathPrefix, value: /match/prefix/}}], backendRefs: [{name: infra-backend-v1, port: 8080}]}
  - {matches: [{path: {type: PathPrefix, value: /match/prefix/one}}], backendRefs: [{name: infra-backend-v2, port: 8080}]}
`)

	s.check(s.serve("same-namespace"), []requestCase{
		{host: "gw.example.net", path: "/match/exact/one", backend: "infra-backend-v3"},
		{host: "gw.example.net", path: "/match/exact", backend: "infra-backend-v2"},
		{host: "gw.example.net", path: "/match", backend: "infra-backend-v1"},
		{host: "gw.example.net", path: "/match/prefix/one/any", backend: "infra-backend-v2"},
		{host: "gw.example.net", path: "/match/prefix/any", backend: "infra-backend-v1"},
		{host: "gw.example.net", path: "/match/any", backend: "infra-backend-v3"},
	})
}

func TestRouteAttachmentReasons(t *testing.T) {
	s := newScenario(t, `
kind: Gateway
metadata: {name: same-namespace, namespace: gateway-conformance-infra}
spec:
  gatewayClassName: cloudflare-tunnel
  listeners:
  - {name: http, port: 80, protocol: HTTP, allowedRoutes: {namespaces: {from: Same}}}
---
kind: HTTPRoute
metadata: {name: wrong-section, namespace: gateway-conformance-infra}
spec:
  parentRefs: [{name: same-namespace, sectionName: http1}]
  rules: [{backendRefs: [{name: infra-backend-v1, port: 8080}]}]
---
kind: HTTPRoute
metadata: {name: other-namespace, namespace: gateway-conformance-web-backend}
spec:
  parentRefs: [{name: same-namespace, namespace: gateway-conformance-infra}]
  rules: [{backendRefs: [{name: web-backend, port: 8080}]}]
---
kind: HTTPRoute
metadata: {name: accepted, namespace: gateway-conformance-infra}
spec:
  parentRefs: [{name: same-namespace}]
  rules: [{backendRefs: [{name: infra-backend-v1, port: 8080}]}]
`)
	assert.Equal(t, gatewayv1.RouteReasonNoMatchingParent, s.parentReason("wrong-section"))
	assert.Equal(t, gatewayv1.RouteReasonAccepted, s.parentReason("accepted"))
	results := s.results[types.NamespacedName{Namespace: "gateway-conformance-web-backend", Name: "other-namespace"}]
	require.Len(t, results, 1)
	assert.Equal(t, gatewayv1.RouteReasonNotAllowedByListeners, results[0].reason)
	assert.Equal(t, map[string]int32{"http": 1}, s.attachedRoutes("same-namespace"))
}
