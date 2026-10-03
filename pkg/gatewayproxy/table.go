// Package gatewayproxy is the data plane of the Gateway API support. One
// proxy runs per Gateway behind the Cloudflare tunnel. The controller
// compiles the routes of a Gateway into a Table, the proxy serves it.
package gatewayproxy

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// TableConfigMapKey is the ConfigMap data key holding the JSON encoded Table.
const TableConfigMapKey = "table.json"

// Table is the routing table of one Gateway. The controller sorts Routes by
// precedence, the proxy takes the first Route that matches a request.
type Table struct {
	Routes []Route `json:"routes"`
}

// Route is one HTTPRoute match on one hostname, with everything needed to
// answer the request.
type Route struct {
	// Protocol is "http" or "https", the protocol of the listener the route
	// came from, kept for logs. Requests do not select listeners by
	// protocol: the Cloudflare edge serves every Gateway address over both
	// schemes.
	Protocol string `json:"protocol"`
	// Hostname is an exact hostname, a wildcard like "*.example.com", or
	// empty to match any host.
	Hostname    string       `json:"hostname,omitempty"`
	Path        PathMatch    `json:"path"`
	Method      string       `json:"method,omitempty"`
	Headers     []ValueMatch `json:"headers,omitempty"`
	QueryParams []ValueMatch `json:"queryParams,omitempty"`

	RequestHeaderModifier *HeaderModifier `json:"requestHeaderModifier,omitempty"`
	Redirect              *Redirect       `json:"redirect,omitempty"`

	// Backends receive the request, picked at random by weight. No
	// backends and no redirect means the rule answers 500.
	Backends []Backend `json:"backends,omitempty"`

	// RouteName identifies the HTTPRoute for logs, as namespace/name.
	RouteName string `json:"routeName,omitempty"`
}

// Path match types, the same values as Gateway API.
const (
	PathMatchExact             = "Exact"
	PathMatchPathPrefix        = "PathPrefix"
	PathMatchRegularExpression = "RegularExpression"
)

type PathMatch struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// ValueMatchRegularExpression is the ValueMatch type for regular expressions,
// an empty type means an exact match.
const ValueMatchRegularExpression = "RegularExpression"

// ValueMatch matches a header or query parameter by name, with type Exact
// (default) or RegularExpression.
type ValueMatch struct {
	Type  string `json:"type,omitempty"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type HeaderModifier struct {
	Set    []Header `json:"set,omitempty"`
	Add    []Header `json:"add,omitempty"`
	Remove []string `json:"remove,omitempty"`
}

// Redirect answers with a redirect instead of forwarding. Empty fields keep
// the value of the incoming request.
type Redirect struct {
	Scheme     string `json:"scheme,omitempty"`
	Hostname   string `json:"hostname,omitempty"`
	Port       int    `json:"port,omitempty"`
	StatusCode int    `json:"statusCode"`
}

type Backend struct {
	// Addresses are host:port pairs, one is picked at random per request.
	Addresses []string `json:"addresses,omitempty"`
	Weight    int32    `json:"weight"`
	// Invalid backends could not be resolved, requests picking them get 500.
	Invalid bool `json:"invalid,omitempty"`
}

// EncodeTable returns the JSON form stored in the ConfigMap.
func EncodeTable(table Table) (string, error) {
	data, err := json.Marshal(table)
	if err != nil {
		return "", errors.Wrap(err, "encode route table")
	}
	return string(data), nil
}

// DecodeTable parses the JSON form stored in the ConfigMap.
func DecodeTable(data string) (*Table, error) {
	table := &Table{}
	if err := json.Unmarshal([]byte(data), table); err != nil {
		return nil, errors.Wrap(err, "decode route table")
	}
	return table, nil
}
