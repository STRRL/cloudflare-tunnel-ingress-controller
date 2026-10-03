package controller

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/gatewayproxy"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// This file holds the pure parts of the Gateway API support: hostname
// intersection, route attachment, reference checks and compiling routes into
// the proxy table. Everything works on a clusterSnapshot, nothing here talks
// to the API server.

// clusterSnapshot is the cluster state one reconcile works on.
type clusterSnapshot struct {
	namespaces      map[string]*corev1.Namespace
	services        map[types.NamespacedName]*corev1.Service
	endpointSlices  map[types.NamespacedName][]discoveryv1.EndpointSlice
	referenceGrants []gatewayv1.ReferenceGrant
	clusterDomain   string
}

// listenerState is the computed state of one Gateway listener.
type listenerState struct {
	listener gatewayv1.Listener
	// accepted is false when the protocol is not supported
	accepted       bool
	supportedKinds []gatewayv1.RouteGroupKind
	// refsReason is empty when all references resolved, otherwise the
	// ResolvedRefs reason
	refsReason     gatewayv1.ListenerConditionReason
	refsMessage    string
	attachedRoutes int32
}

// gatewayState is the computed state of one Gateway.
type gatewayState struct {
	gateway   *gatewayv1.Gateway
	address   string
	accepted  bool
	reason    gatewayv1.GatewayConditionReason
	message   string
	listeners []*listenerState
	// attachments are the accepted routes with the listeners they attach to
	attachments []routeAttachment
}

type routeAttachment struct {
	route *gatewayv1.HTTPRoute
	// hostnames per attached listener index, "" means any host
	hostnames map[int][]string
}

// parentResult is the outcome of one route parentRef that points at one of
// our Gateways.
type parentResult struct {
	ref      gatewayv1.ParentReference
	accepted bool
	reason   gatewayv1.RouteConditionReason
	message  string
}

const (
	gatewayGroup = gatewayv1.GroupName
	kindGateway  = "Gateway"
	kindHTTPRte  = "HTTPRoute"
	kindService  = "Service"
	kindSecret   = "Secret"
)

// gatewayAddressLabel returns the first label of a Gateway address:
// <name>-<namespace><suffix>. The address is this label plus the base
// domain, one level below the zone, so the Cloudflare Universal SSL
// certificate covers it. A label longer than 63 characters keeps the suffix,
// the name part is cut and gets a hash, so it stays unique.
func gatewayAddressLabel(namespace string, name string, suffix string) string {
	label := name + "-" + namespace
	if len(label)+len(suffix) <= 63 {
		return label + suffix
	}
	keep := max(63-len(suffix)-9, 1)
	return strings.TrimRight(label[:keep], "-") + "-" + shortHash(namespace+"/"+name) + suffix
}

// shortHash returns 8 hex characters of sha256.
func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:8]
}

// hostnameIntersection returns the hostname both a listener and a route
// hostname accept, and false when they do not overlap. Empty means any
// host.
func hostnameIntersection(listenerHostname string, routeHostname string) (string, bool) {
	if listenerHostname == "" {
		return routeHostname, true
	}
	if routeHostname == "" {
		return listenerHostname, true
	}
	listenerWildcard := strings.HasPrefix(listenerHostname, "*.")
	routeWildcard := strings.HasPrefix(routeHostname, "*.")
	switch {
	case !listenerWildcard && !routeWildcard:
		return routeHostname, listenerHostname == routeHostname
	case listenerWildcard && !routeWildcard:
		return routeHostname, wildcardCovers(listenerHostname, routeHostname)
	case !listenerWildcard && routeWildcard:
		return listenerHostname, wildcardCovers(routeHostname, listenerHostname)
	default:
		// both wildcards: the narrower one is the intersection
		if listenerHostname == routeHostname || strings.HasSuffix(routeHostname, listenerHostname[1:]) {
			return routeHostname, true
		}
		if strings.HasSuffix(listenerHostname, routeHostname[1:]) {
			return listenerHostname, true
		}
		return "", false
	}
}

// wildcardCovers reports whether "*.example.com" covers host, which needs at
// least one label in front of example.com.
func wildcardCovers(wildcard string, host string) bool {
	suffix := wildcard[1:]
	return strings.HasSuffix(host, suffix) && len(host) > len(suffix)
}

// listenerRouteHostnames returns the hostnames a route serves on a listener,
// nil when they do not intersect.
func listenerRouteHostnames(listener gatewayv1.Listener, route *gatewayv1.HTTPRoute) []string {
	listenerHostname := ""
	if listener.Hostname != nil {
		listenerHostname = strings.ToLower(string(*listener.Hostname))
	}
	if len(route.Spec.Hostnames) == 0 {
		return []string{listenerHostname}
	}
	var result []string
	for _, item := range route.Spec.Hostnames {
		if hostname, ok := hostnameIntersection(listenerHostname, strings.ToLower(string(item))); ok && !slices.Contains(result, hostname) {
			result = append(result, hostname)
		}
	}
	return result
}

// normalizeParentRef fills group, kind and namespace, so status entries
// always carry them explicitly.
func normalizeParentRef(ref gatewayv1.ParentReference, routeNamespace string) gatewayv1.ParentReference {
	result := *ref.DeepCopy()
	if result.Group == nil {
		result.Group = new(gatewayv1.Group(gatewayGroup))
	}
	if result.Kind == nil {
		result.Kind = new(gatewayv1.Kind(kindGateway))
	}
	if result.Namespace == nil {
		result.Namespace = new(gatewayv1.Namespace(routeNamespace))
	}
	return result
}

// isGatewayParentRef reports whether the parentRef points at a Gateway.
func isGatewayParentRef(ref gatewayv1.ParentReference) bool {
	return string(*ref.Group) == gatewayGroup && string(*ref.Kind) == kindGateway
}

// attachRoute decides whether a route attaches to a Gateway through one
// parentRef, and to which listeners. The parentRef must be normalized.
func attachRoute(route *gatewayv1.HTTPRoute, ref gatewayv1.ParentReference, state *gatewayState, snapshot *clusterSnapshot) (parentResult, map[int][]string) {
	result := parentResult{ref: ref}

	var candidates []int
	for i, item := range state.listeners {
		if ref.SectionName != nil && item.listener.Name != *ref.SectionName {
			continue
		}
		if ref.Port != nil && item.listener.Port != *ref.Port {
			continue
		}
		candidates = append(candidates, i)
	}
	if len(candidates) == 0 {
		result.reason = gatewayv1.RouteReasonNoMatchingParent
		result.message = "no listener matches the parentRef sectionName or port"
		return result, nil
	}

	var allowed []int
	for _, i := range candidates {
		item := state.listeners[i]
		if item.accepted && listenerAllowsKind(item) && listenerAllowsNamespace(item.listener, state.gateway.Namespace, route.Namespace, snapshot) {
			allowed = append(allowed, i)
		}
	}
	if len(allowed) == 0 {
		result.reason = gatewayv1.RouteReasonNotAllowedByListeners
		result.message = "no listener allows this route kind or namespace"
		return result, nil
	}

	attached := map[int][]string{}
	for _, i := range allowed {
		if hostnames := listenerRouteHostnames(state.listeners[i].listener, route); len(hostnames) > 0 {
			attached[i] = hostnames
		}
	}
	if len(attached) == 0 {
		result.reason = gatewayv1.RouteReasonNoMatchingListenerHostname
		result.message = "no listener hostname intersects the route hostnames"
		return result, nil
	}

	result.accepted = true
	result.reason = gatewayv1.RouteReasonAccepted
	result.message = "route is accepted"
	return result, attached
}

// attachRoutes attaches every route to the Gateways it references and counts
// the attached routes per listener. Routes without a parentRef to one of the
// given accepted Gateways are missing from the result.
func attachRoutes(states map[types.NamespacedName]*gatewayState, routes []gatewayv1.HTTPRoute, snapshot *clusterSnapshot) map[types.NamespacedName][]parentResult {
	routeResults := map[types.NamespacedName][]parentResult{}
	for i := range routes {
		route := &routes[i]
		if route.DeletionTimestamp != nil {
			continue
		}
		var results []parentResult
		for _, rawRef := range route.Spec.ParentRefs {
			ref := normalizeParentRef(rawRef, route.Namespace)
			if !isGatewayParentRef(ref) {
				continue
			}
			state, ok := states[types.NamespacedName{Namespace: string(*ref.Namespace), Name: string(ref.Name)}]
			if !ok || !state.accepted {
				continue
			}
			result, attached := attachRoute(route, ref, state, snapshot)
			results = append(results, result)
			if result.accepted {
				state.attachments = append(state.attachments, routeAttachment{route: route, hostnames: attached})
			}
		}
		if len(results) > 0 {
			routeResults[types.NamespacedName{Namespace: route.Namespace, Name: route.Name}] = results
		}
	}

	// a route counts once per listener, even through several parentRefs
	for _, state := range states {
		counted := map[int]map[*gatewayv1.HTTPRoute]bool{}
		for _, attachment := range state.attachments {
			for listenerIndex := range attachment.hostnames {
				if counted[listenerIndex] == nil {
					counted[listenerIndex] = map[*gatewayv1.HTTPRoute]bool{}
				}
				counted[listenerIndex][attachment.route] = true
			}
		}
		for listenerIndex, items := range counted {
			state.listeners[listenerIndex].attachedRoutes = int32(len(items))
		}
	}
	return routeResults
}

func listenerAllowsKind(item *listenerState) bool {
	return slices.ContainsFunc(item.supportedKinds, func(kind gatewayv1.RouteGroupKind) bool {
		return kind.Kind == kindHTTPRte
	})
}

func listenerAllowsNamespace(listener gatewayv1.Listener, gatewayNamespace string, routeNamespace string, snapshot *clusterSnapshot) bool {
	from := gatewayv1.NamespacesFromSame
	var selector *metav1.LabelSelector
	if listener.AllowedRoutes != nil && listener.AllowedRoutes.Namespaces != nil {
		if listener.AllowedRoutes.Namespaces.From != nil {
			from = *listener.AllowedRoutes.Namespaces.From
		}
		selector = listener.AllowedRoutes.Namespaces.Selector
	}
	switch from {
	case gatewayv1.NamespacesFromAll:
		return true
	case gatewayv1.NamespacesFromSelector:
		namespace, ok := snapshot.namespaces[routeNamespace]
		if !ok || selector == nil {
			return false
		}
		parsed, err := metav1.LabelSelectorAsSelector(selector)
		if err != nil {
			return false
		}
		return parsed.Matches(labels.Set(namespace.Labels))
	default:
		return gatewayNamespace == routeNamespace
	}
}

// referenceGrantAllows reports whether a ReferenceGrant in the target
// namespace allows the reference.
func referenceGrantAllows(grants []gatewayv1.ReferenceGrant, fromGroup string, fromKind string, fromNamespace string, toGroup string, toKind string, toNamespace string, toName string) bool {
	for _, grant := range grants {
		if grant.Namespace != toNamespace {
			continue
		}
		fromMatches := slices.ContainsFunc(grant.Spec.From, func(from gatewayv1.ReferenceGrantFrom) bool {
			return string(from.Group) == fromGroup && string(from.Kind) == fromKind && string(from.Namespace) == fromNamespace
		})
		toMatches := slices.ContainsFunc(grant.Spec.To, func(to gatewayv1.ReferenceGrantTo) bool {
			return string(to.Group) == toGroup && string(to.Kind) == toKind && (to.Name == nil || string(*to.Name) == toName)
		})
		if fromMatches && toMatches {
			return true
		}
	}
	return false
}

// backendResult is a resolved backendRef, or the reason it failed.
type backendResult struct {
	reason    gatewayv1.RouteConditionReason
	message   string
	addresses []string
}

// resolveBackend checks one backendRef and returns where to send traffic.
func resolveBackend(route *gatewayv1.HTTPRoute, ref gatewayv1.BackendObjectReference, snapshot *clusterSnapshot) backendResult {
	group := ""
	if ref.Group != nil {
		group = string(*ref.Group)
	}
	kind := kindService
	if ref.Kind != nil {
		kind = string(*ref.Kind)
	}
	if group != "" || kind != kindService {
		return backendResult{reason: gatewayv1.RouteReasonInvalidKind, message: fmt.Sprintf("backendRef %s/%s kind %s is not supported", group, kind, ref.Name)}
	}

	namespace := route.Namespace
	if ref.Namespace != nil {
		namespace = string(*ref.Namespace)
	}
	if namespace != route.Namespace && !referenceGrantAllows(snapshot.referenceGrants, gatewayGroup, kindHTTPRte, route.Namespace, "", kindService, namespace, string(ref.Name)) {
		return backendResult{reason: gatewayv1.RouteReasonRefNotPermitted, message: fmt.Sprintf("backendRef to service %s/%s is not permitted by any ReferenceGrant", namespace, ref.Name)}
	}

	service, ok := snapshot.services[types.NamespacedName{Namespace: namespace, Name: string(ref.Name)}]
	if !ok {
		return backendResult{reason: gatewayv1.RouteReasonBackendNotFound, message: fmt.Sprintf("service %s/%s not found", namespace, ref.Name)}
	}
	if ref.Port == nil {
		return backendResult{reason: gatewayv1.RouteReasonUnsupportedValue, message: fmt.Sprintf("backendRef to service %s/%s has no port", namespace, ref.Name)}
	}
	port := int32(*ref.Port)

	if service.Spec.ClusterIP != corev1.ClusterIPNone {
		address := net.JoinHostPort(fmt.Sprintf("%s.%s.svc.%s", service.Name, service.Namespace, snapshot.clusterDomain), strconv.Itoa(int(port)))
		return backendResult{addresses: []string{address}}
	}

	// headless services have no cluster IP, send traffic to the endpoints
	// directly, on the endpoint port that belongs to the service port
	var portName string
	found := false
	for _, item := range service.Spec.Ports {
		if item.Port == port {
			portName = item.Name
			found = true
		}
	}
	if !found {
		return backendResult{reason: gatewayv1.RouteReasonBackendNotFound, message: fmt.Sprintf("service %s/%s has no port %d", namespace, ref.Name, port)}
	}
	var addresses []string
	for _, slice := range snapshot.endpointSlices[types.NamespacedName{Namespace: namespace, Name: service.Name}] {
		var endpointPort int32
		for _, item := range slice.Ports {
			if item.Port != nil && ptrValue(item.Name) == portName {
				endpointPort = *item.Port
			}
		}
		if endpointPort == 0 {
			continue
		}
		for _, endpoint := range slice.Endpoints {
			if endpoint.Conditions.Ready != nil && !*endpoint.Conditions.Ready {
				continue
			}
			for _, address := range endpoint.Addresses {
				addresses = append(addresses, net.JoinHostPort(address, strconv.Itoa(int(endpointPort))))
			}
		}
	}
	slices.Sort(addresses)
	return backendResult{addresses: addresses}
}

func ptrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// routeResolvedRefs returns the ResolvedRefs reason of a route, empty when
// every backendRef resolves.
func routeResolvedRefs(route *gatewayv1.HTTPRoute, snapshot *clusterSnapshot) (gatewayv1.RouteConditionReason, string) {
	for _, rule := range route.Spec.Rules {
		for _, ref := range rule.BackendRefs {
			if result := resolveBackend(route, ref.BackendObjectReference, snapshot); result.reason != "" {
				return result.reason, result.message
			}
		}
	}
	return "", ""
}

// compiledRoute is a table route with the data needed to sort it.
type compiledRoute struct {
	route      gatewayproxy.Route
	created    metav1.Time
	routeKey   string
	ruleIndex  int
	matchIndex int
}

// compileTable turns the attached routes of a Gateway into the proxy table,
// sorted by Gateway API match precedence.
func compileTable(state *gatewayState, snapshot *clusterSnapshot) gatewayproxy.Table {
	var compiled []compiledRoute
	for _, attachment := range state.attachments {
		route := attachment.route
		routeKey := route.Namespace + "/" + route.Name
		for listenerIndex, hostnames := range attachment.hostnames {
			protocol := "http"
			if state.listeners[listenerIndex].listener.Protocol == gatewayv1.HTTPSProtocolType {
				protocol = "https"
			}
			for ruleIndex, rule := range route.Spec.Rules {
				matches := rule.Matches
				if len(matches) == 0 {
					matches = []gatewayv1.HTTPRouteMatch{{}}
				}
				for matchIndex, match := range matches {
					for _, hostname := range hostnames {
						item := compileRule(rule, match, route, snapshot)
						item.Protocol = protocol
						item.Hostname = hostname
						item.RouteName = routeKey
						compiled = append(compiled, compiledRoute{
							route:      item,
							created:    route.CreationTimestamp,
							routeKey:   routeKey,
							ruleIndex:  ruleIndex,
							matchIndex: matchIndex,
						})
					}
				}
			}
		}
	}

	slices.SortStableFunc(compiled, compareRoutePrecedence)

	table := gatewayproxy.Table{Routes: []gatewayproxy.Route{}}
	for _, item := range compiled {
		// the same rule can attach through several listeners with the
		// same protocol and hostname, keep one copy
		if len(table.Routes) > 0 && reflect.DeepEqual(table.Routes[len(table.Routes)-1], item.route) {
			continue
		}
		table.Routes = append(table.Routes, item.route)
	}
	return table
}

func compileRule(rule gatewayv1.HTTPRouteRule, match gatewayv1.HTTPRouteMatch, route *gatewayv1.HTTPRoute, snapshot *clusterSnapshot) gatewayproxy.Route {
	result := gatewayproxy.Route{
		Path: gatewayproxy.PathMatch{Type: gatewayproxy.PathMatchPathPrefix, Value: "/"},
	}
	if match.Path != nil {
		if match.Path.Type != nil {
			result.Path.Type = string(*match.Path.Type)
		}
		if match.Path.Value != nil {
			result.Path.Value = *match.Path.Value
		}
	}
	if match.Method != nil {
		result.Method = string(*match.Method)
	}
	for _, header := range match.Headers {
		item := gatewayproxy.ValueMatch{Name: string(header.Name), Value: header.Value}
		if header.Type != nil && *header.Type == gatewayv1.HeaderMatchRegularExpression {
			item.Type = gatewayproxy.ValueMatchRegularExpression
		}
		result.Headers = append(result.Headers, item)
	}
	for _, param := range match.QueryParams {
		item := gatewayproxy.ValueMatch{Name: string(param.Name), Value: param.Value}
		if param.Type != nil && *param.Type == gatewayv1.QueryParamMatchRegularExpression {
			item.Type = gatewayproxy.ValueMatchRegularExpression
		}
		result.QueryParams = append(result.QueryParams, item)
	}

	for _, filter := range rule.Filters {
		switch filter.Type {
		case gatewayv1.HTTPRouteFilterRequestHeaderModifier:
			if filter.RequestHeaderModifier != nil {
				modifier := &gatewayproxy.HeaderModifier{Remove: filter.RequestHeaderModifier.Remove}
				for _, header := range filter.RequestHeaderModifier.Set {
					modifier.Set = append(modifier.Set, gatewayproxy.Header{Name: string(header.Name), Value: header.Value})
				}
				for _, header := range filter.RequestHeaderModifier.Add {
					modifier.Add = append(modifier.Add, gatewayproxy.Header{Name: string(header.Name), Value: header.Value})
				}
				result.RequestHeaderModifier = modifier
			}
		case gatewayv1.HTTPRouteFilterRequestRedirect:
			if filter.RequestRedirect != nil {
				redirect := &gatewayproxy.Redirect{StatusCode: 302}
				if filter.RequestRedirect.Scheme != nil {
					redirect.Scheme = *filter.RequestRedirect.Scheme
				}
				if filter.RequestRedirect.Hostname != nil {
					redirect.Hostname = string(*filter.RequestRedirect.Hostname)
				}
				if filter.RequestRedirect.Port != nil {
					redirect.Port = int(*filter.RequestRedirect.Port)
				}
				if filter.RequestRedirect.StatusCode != nil {
					redirect.StatusCode = *filter.RequestRedirect.StatusCode
				}
				result.Redirect = redirect
			}
		default:
			// other filters are not supported, the rule answers 500 instead
			// of silently ignoring the filter
			result.Backends = nil
			result.Redirect = nil
			result.RequestHeaderModifier = nil
			return result
		}
	}
	if result.Redirect != nil {
		return result
	}

	for _, ref := range rule.BackendRefs {
		weight := int32(1)
		if ref.Weight != nil {
			weight = *ref.Weight
		}
		resolved := resolveBackend(route, ref.BackendObjectReference, snapshot)
		result.Backends = append(result.Backends, gatewayproxy.Backend{
			Addresses: resolved.addresses,
			Weight:    weight,
			Invalid:   resolved.reason != "",
		})
	}
	return result
}

// compareRoutePrecedence orders routes as Gateway API requires: Exact path,
// then longest prefix, then method, then most headers, then most query
// params, then oldest route, then route name, then rule and match order.
func compareRoutePrecedence(a compiledRoute, b compiledRoute) int {
	return cmp.Or(
		cmp.Compare(pathTypeRank(a.route.Path.Type), pathTypeRank(b.route.Path.Type)),
		-cmp.Compare(len(a.route.Path.Value), len(b.route.Path.Value)),
		-cmp.Compare(boolRank(a.route.Method != ""), boolRank(b.route.Method != "")),
		-cmp.Compare(len(a.route.Headers), len(b.route.Headers)),
		-cmp.Compare(len(a.route.QueryParams), len(b.route.QueryParams)),
		a.created.Compare(b.created.Time),
		cmp.Compare(a.routeKey, b.routeKey),
		cmp.Compare(a.ruleIndex, b.ruleIndex),
		cmp.Compare(a.matchIndex, b.matchIndex),
		cmp.Compare(a.route.Protocol, b.route.Protocol),
		cmp.Compare(a.route.Hostname, b.route.Hostname),
	)
}

func pathTypeRank(pathType string) int {
	switch pathType {
	case gatewayproxy.PathMatchExact:
		return 0
	case gatewayproxy.PathMatchPathPrefix:
		return 1
	default:
		return 2
	}
}

func boolRank(value bool) int {
	if value {
		return 1
	}
	return 0
}
