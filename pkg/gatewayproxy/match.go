package gatewayproxy

import (
	"net"
	"net/http"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

// matcher is a Table ready to serve: regular expressions are compiled once.
type matcher struct {
	table   *Table
	regexps map[string]*regexp.Regexp
}

func newMatcher(table *Table) (*matcher, error) {
	result := &matcher{table: table, regexps: map[string]*regexp.Regexp{}}
	addRegexp := func(pattern string) error {
		if _, ok := result.regexps[pattern]; ok {
			return nil
		}
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return errors.Wrapf(err, "compile regular expression %q", pattern)
		}
		result.regexps[pattern] = compiled
		return nil
	}
	for _, route := range table.Routes {
		if route.Path.Type == PathMatchRegularExpression {
			if err := addRegexp(route.Path.Value); err != nil {
				return nil, err
			}
		}
		for _, items := range [][]ValueMatch{route.Headers, route.QueryParams} {
			for _, item := range items {
				if item.Type == ValueMatchRegularExpression {
					if err := addRegexp(item.Value); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	return result, nil
}

// requestHost returns the Host header without port, in lower case.
func requestHost(r *http.Request) string {
	host := r.Host
	if withoutPort, _, err := net.SplitHostPort(host); err == nil {
		host = withoutPort
	}
	return strings.ToLower(strings.TrimSuffix(host, "."))
}

// hostnameMatches reports whether a route hostname covers the host. Empty
// matches any host, "*.example.com" matches one or more labels in front of
// example.com but not example.com itself.
func hostnameMatches(hostname string, host string) bool {
	if hostname == "" {
		return true
	}
	if suffix, ok := strings.CutPrefix(hostname, "*"); ok {
		return strings.HasSuffix(host, suffix) && len(host) > len(suffix)
	}
	return hostname == host
}

// hostnameSpecificity ranks matching hostnames: exact first, then wildcards
// with more labels, then the empty hostname that matches anything.
func hostnameSpecificity(hostname string) int {
	if hostname == "" {
		return 0
	}
	if strings.HasPrefix(hostname, "*.") {
		return 1 + strings.Count(hostname, ".")
	}
	return 1000
}

// find returns the first route that matches the request. Like a virtual host,
// the most specific hostname matching the request host is chosen first, then
// only the routes of that hostname are considered, in table order.
func (m *matcher) find(r *http.Request) *Route {
	host := requestHost(r)

	bestHostname := ""
	bestSpecificity := -1
	for i := range m.table.Routes {
		route := &m.table.Routes[i]
		if !hostnameMatches(route.Hostname, host) {
			continue
		}
		if specificity := hostnameSpecificity(route.Hostname); specificity > bestSpecificity {
			bestHostname = route.Hostname
			bestSpecificity = specificity
		}
	}
	if bestSpecificity < 0 {
		return nil
	}

	for i := range m.table.Routes {
		route := &m.table.Routes[i]
		if route.Hostname != bestHostname {
			continue
		}
		if m.routeMatches(route, r) {
			return route
		}
	}
	return nil
}

func (m *matcher) routeMatches(route *Route, r *http.Request) bool {
	if !m.pathMatches(route.Path, r.URL.Path) {
		return false
	}
	if route.Method != "" && route.Method != r.Method {
		return false
	}
	for _, header := range route.Headers {
		values := r.Header.Values(header.Name)
		if len(values) == 0 || !m.valueMatches(header, strings.Join(values, ",")) {
			return false
		}
	}
	query := r.URL.Query()
	for _, param := range route.QueryParams {
		if !query.Has(param.Name) || !m.valueMatches(param, query.Get(param.Name)) {
			return false
		}
	}
	return true
}

func (m *matcher) valueMatches(match ValueMatch, value string) bool {
	if match.Type == ValueMatchRegularExpression {
		return m.regexps[match.Value].MatchString(value)
	}
	return match.Value == value
}

func (m *matcher) pathMatches(match PathMatch, path string) bool {
	switch match.Type {
	case PathMatchExact:
		return path == match.Value
	case PathMatchRegularExpression:
		return m.regexps[match.Value].MatchString(path)
	default:
		return pathPrefixMatches(match.Value, path)
	}
}

// pathPrefixMatches matches on whole path segments: "/v2" matches "/v2" and
// "/v2/example" but not "/v2example". A trailing slash in the prefix is
// ignored.
func pathPrefixMatches(prefix string, path string) bool {
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" {
		return true
	}
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}
