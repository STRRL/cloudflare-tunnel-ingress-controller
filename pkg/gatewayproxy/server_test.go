package gatewayproxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// echoed is what a test backend saw.
type echoed struct {
	Backend string      `json:"backend"`
	Host    string      `json:"host"`
	Path    string      `json:"path"`
	Headers http.Header `json:"headers"`
}

// startBackend starts a real HTTP backend that answers with what it received.
func startBackend(t *testing.T, name string) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(echoed{Backend: name, Host: r.Host, Path: r.URL.Path, Headers: r.Header})
	}))
	t.Cleanup(server.Close)
	return strings.TrimPrefix(server.URL, "http://")
}

func startProxy(t *testing.T, table Table) *httptest.Server {
	t.Helper()
	server := NewServer(logr.Discard())
	require.NoError(t, server.SetTable(&table))
	proxy := httptest.NewServer(server)
	t.Cleanup(proxy.Close)
	return proxy
}

// send makes a request through the proxy with the given Host and headers and
// returns the status code and, for 200, what the backend saw.
func send(t *testing.T, proxyURL string, method string, host string, path string, headers map[string]string) (int, *echoed, http.Header) {
	t.Helper()
	request, err := http.NewRequest(method, proxyURL+path, nil)
	require.NoError(t, err)
	request.Host = host
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	require.NoError(t, err)
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return response.StatusCode, nil, response.Header
	}
	result := &echoed{}
	require.NoError(t, json.NewDecoder(response.Body).Decode(result))
	return response.StatusCode, result, response.Header
}

func backend(address string) []Backend {
	return []Backend{{Addresses: []string{address}, Weight: 1}}
}

func TestServerMatching(t *testing.T) {
	v1 := startBackend(t, "v1")
	v2 := startBackend(t, "v2")
	v3 := startBackend(t, "v3")

	// the table is already sorted by precedence, like the controller does
	table := Table{Routes: []Route{
		{Protocol: "http", Path: PathMatch{Type: PathMatchExact, Value: "/exact"}, Backends: backend(v3)},
		{Protocol: "http", Path: PathMatch{Type: PathMatchPathPrefix, Value: "/v2"}, Backends: backend(v2)},
		{Protocol: "http", Path: PathMatch{Type: PathMatchPathPrefix, Value: "/"}, Headers: []ValueMatch{{Name: "version", Value: "two"}, {Name: "color", Value: "orange"}}, Backends: backend(v1)},
		{Protocol: "http", Path: PathMatch{Type: PathMatchPathPrefix, Value: "/"}, Headers: []ValueMatch{{Name: "version", Value: "two"}}, Backends: backend(v2)},
		{Protocol: "http", Path: PathMatch{Type: PathMatchPathPrefix, Value: "/"}, Method: "POST", Backends: backend(v3)},
		{Protocol: "http", Path: PathMatch{Type: PathMatchPathPrefix, Value: "/"}, QueryParams: []ValueMatch{{Type: ValueMatchRegularExpression, Name: "id", Value: "^[0-9]+$"}}, Backends: backend(v3)},
		{Protocol: "http", Path: PathMatch{Type: PathMatchPathPrefix, Value: "/"}, Backends: backend(v1)},
	}}
	proxy := startProxy(t, table)

	cases := []struct {
		name    string
		method  string
		path    string
		headers map[string]string
		backend string
	}{
		{name: "exact path", path: "/exact", backend: "v3"},
		{name: "exact path does not match a longer path", path: "/exact/more", backend: "v1"},
		{name: "prefix matches the prefix itself", path: "/v2", backend: "v2"},
		{name: "prefix matches below the prefix", path: "/v2/example", backend: "v2"},
		{name: "prefix matches with trailing slash", path: "/v2/", backend: "v2"},
		{name: "prefix needs a segment boundary", path: "/v2example", backend: "v1"},
		{name: "more headers win", path: "/", headers: map[string]string{"Version": "two", "Color": "orange"}, backend: "v1"},
		{name: "one header", path: "/", headers: map[string]string{"Version": "two"}, backend: "v2"},
		{name: "method", method: "POST", path: "/", backend: "v3"},
		{name: "query parameter regular expression", path: "/?id=42", backend: "v3"},
		{name: "query parameter regular expression no match", path: "/?id=abc", backend: "v1"},
		{name: "listener protocol does not filter requests", path: "/", headers: map[string]string{"X-Forwarded-Proto": "https"}, backend: "v1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			method := tc.method
			if method == "" {
				method = http.MethodGet
			}
			status, result, _ := send(t, proxy.URL, method, "gateway.example.net", tc.path, tc.headers)
			require.Equal(t, http.StatusOK, status)
			assert.Equal(t, tc.backend, result.Backend)
			assert.Equal(t, "gateway.example.net", result.Host, "the backend must see the original Host")
		})
	}
}

func TestServerNoMatchAndInvalidBackends(t *testing.T) {
	v1 := startBackend(t, "v1")
	proxy := startProxy(t, Table{Routes: []Route{
		{Protocol: "http", Path: PathMatch{Type: PathMatchExact, Value: "/ok"}, Backends: backend(v1)},
		{Protocol: "http", Path: PathMatch{Type: PathMatchExact, Value: "/no-backends"}},
		{Protocol: "http", Path: PathMatch{Type: PathMatchExact, Value: "/invalid"}, Backends: []Backend{{Weight: 1, Invalid: true}}},
		{Protocol: "http", Path: PathMatch{Type: PathMatchExact, Value: "/zero-weight"}, Backends: []Backend{{Addresses: []string{v1}, Weight: 0}}},
	}})

	cases := map[string]int{
		"/ok":          http.StatusOK,
		"/missing":     http.StatusNotFound,
		"/no-backends": http.StatusInternalServerError,
		"/invalid":     http.StatusInternalServerError,
		"/zero-weight": http.StatusInternalServerError,
	}
	for path, expected := range cases {
		status, _, _ := send(t, proxy.URL, http.MethodGet, "gateway.example.net", path, nil)
		assert.Equal(t, expected, status, path)
	}
}

func TestServerForwardedHeaders(t *testing.T) {
	v1 := startBackend(t, "v1")
	proxy := startProxy(t, Table{Routes: []Route{{Protocol: "http", Path: PathMatch{Type: PathMatchPathPrefix, Value: "/"}, Backends: backend(v1)}}})

	// cloudflared forwards the edge scheme and the client address
	_, result, _ := send(t, proxy.URL, http.MethodGet, "gateway.example.net", "/", map[string]string{
		"X-Forwarded-Proto": "https",
		"X-Forwarded-For":   "203.0.113.7",
	})
	assert.Equal(t, "https", result.Headers.Get("X-Forwarded-Proto"))
	assert.Equal(t, "gateway.example.net", result.Headers.Get("X-Forwarded-Host"))
	assert.True(t, strings.HasPrefix(result.Headers.Get("X-Forwarded-For"), "203.0.113.7, "), result.Headers.Get("X-Forwarded-For"))

	// without a forwarded scheme the request was plain http
	_, result, _ = send(t, proxy.URL, http.MethodGet, "gateway.example.net", "/", nil)
	assert.Equal(t, "http", result.Headers.Get("X-Forwarded-Proto"))
}

func TestServerRequestHeaderModifier(t *testing.T) {
	v1 := startBackend(t, "v1")
	proxy := startProxy(t, Table{Routes: []Route{{
		Protocol: "http",
		Path:     PathMatch{Type: PathMatchPathPrefix, Value: "/"},
		RequestHeaderModifier: &HeaderModifier{
			Set:    []Header{{Name: "X-Header-Set", Value: "set-overwrites-values"}},
			Add:    []Header{{Name: "X-Header-Add", Value: "add-appends-values"}},
			Remove: []string{"X-Header-Remove"},
		},
		Backends: backend(v1),
	}}})

	_, result, _ := send(t, proxy.URL, http.MethodGet, "gateway.example.net", "/", map[string]string{
		"x-header-set":    "original",
		"x-header-add":    "original",
		"x-header-remove": "original",
	})
	assert.Equal(t, []string{"set-overwrites-values"}, result.Headers.Values("X-Header-Set"))
	assert.Equal(t, []string{"original", "add-appends-values"}, result.Headers.Values("X-Header-Add"))
	assert.Empty(t, result.Headers.Values("X-Header-Remove"))
}

func TestServerRedirect(t *testing.T) {
	proxy := startProxy(t, Table{Routes: []Route{
		{Protocol: "http", Path: PathMatch{Type: PathMatchPathPrefix, Value: "/hostname"}, Redirect: &Redirect{Hostname: "example.org", StatusCode: 302}},
		{Protocol: "http", Path: PathMatch{Type: PathMatchPathPrefix, Value: "/status"}, Redirect: &Redirect{Hostname: "example.org", StatusCode: 301}},
		{Protocol: "http", Path: PathMatch{Type: PathMatchPathPrefix, Value: "/scheme"}, Redirect: &Redirect{Scheme: "https", StatusCode: 302}},
		{Protocol: "http", Path: PathMatch{Type: PathMatchPathPrefix, Value: "/port"}, Redirect: &Redirect{Port: 8443, StatusCode: 302}},
	}})

	cases := []struct {
		path     string
		status   int
		location string
	}{
		{path: "/hostname/a?b=c", status: 302, location: "http://example.org/hostname/a?b=c"},
		{path: "/status", status: 301, location: "http://example.org/status"},
		{path: "/scheme", status: 302, location: "https://gateway.example.net/scheme"},
		{path: "/port", status: 302, location: "http://gateway.example.net:8443/port"},
	}
	for _, tc := range cases {
		status, _, header := send(t, proxy.URL, http.MethodGet, "gateway.example.net:80", tc.path, nil)
		assert.Equal(t, tc.status, status, tc.path)
		assert.Equal(t, tc.location, header.Get("Location"), tc.path)
	}

	// without an explicit scheme the redirect keeps the scheme the client
	// used at the edge
	status, _, header := send(t, proxy.URL, http.MethodGet, "gateway.example.net", "/hostname", map[string]string{"X-Forwarded-Proto": "https"})
	assert.Equal(t, 302, status)
	assert.Equal(t, "https://example.org/hostname", header.Get("Location"))
}

func TestServerWeights(t *testing.T) {
	v1 := startBackend(t, "v1")
	v2 := startBackend(t, "v2")
	v3 := startBackend(t, "v3")
	proxy := startProxy(t, Table{Routes: []Route{{
		Protocol: "http",
		Path:     PathMatch{Type: PathMatchPathPrefix, Value: "/"},
		Backends: []Backend{
			{Addresses: []string{v1}, Weight: 70},
			{Addresses: []string{v2}, Weight: 30},
			{Addresses: []string{v3}, Weight: 0},
		},
	}}})

	counts := map[string]int{}
	const total = 1000
	for range total {
		_, result, _ := send(t, proxy.URL, http.MethodGet, "gateway.example.net", "/", nil)
		counts[result.Backend]++
	}
	assert.InDelta(t, 0.7, float64(counts["v1"])/total, 0.06)
	assert.InDelta(t, 0.3, float64(counts["v2"])/total, 0.06)
	assert.Zero(t, counts["v3"])
}

func TestServerVirtualHosts(t *testing.T) {
	v1 := startBackend(t, "v1")
	v2 := startBackend(t, "v2")
	v3 := startBackend(t, "v3")
	proxy := startProxy(t, Table{Routes: []Route{
		{Protocol: "http", Hostname: "very.specific.com", Path: PathMatch{Type: PathMatchPathPrefix, Value: "/s1"}, Backends: backend(v1)},
		{Protocol: "http", Hostname: "*.wildcard.io", Path: PathMatch{Type: PathMatchPathPrefix, Value: "/"}, Backends: backend(v2)},
		{Protocol: "http", Path: PathMatch{Type: PathMatchPathPrefix, Value: "/"}, Backends: backend(v3)},
	}})

	cases := []struct {
		host    string
		path    string
		status  int
		backend string
	}{
		{host: "very.specific.com", path: "/s1", status: 200, backend: "v1"},
		{host: "very.specific.com:1234", path: "/s1", status: 200, backend: "v1"},
		// the most specific hostname wins and there is no fall through
		{host: "very.specific.com", path: "/other", status: 404},
		{host: "foo.wildcard.io", path: "/", status: 200, backend: "v2"},
		{host: "foo.bar.wildcard.io", path: "/", status: 200, backend: "v2"},
		{host: "wildcard.io", path: "/", status: 200, backend: "v3"},
		{host: "other.com", path: "/", status: 200, backend: "v3"},
	}
	for _, tc := range cases {
		status, result, _ := send(t, proxy.URL, http.MethodGet, tc.host, tc.path, nil)
		require.Equal(t, tc.status, status, tc.host+tc.path)
		if tc.status == http.StatusOK {
			assert.Equal(t, tc.backend, result.Backend, tc.host+tc.path)
		}
	}
}

// TestServerTableSwapUnderLoad replaces the table while requests are in
// flight: every request must get an answer from either the old or the new
// table, never an error.
func TestServerTableSwapUnderLoad(t *testing.T) {
	v1 := startBackend(t, "v1")
	v2 := startBackend(t, "v2")
	tableFor := func(address string) *Table {
		return &Table{Routes: []Route{{Protocol: "http", Path: PathMatch{Type: PathMatchPathPrefix, Value: "/"}, Backends: backend(address)}}}
	}
	server := NewServer(logr.Discard())
	require.NoError(t, server.SetTable(tableFor(v1)))
	proxy := httptest.NewServer(server)
	t.Cleanup(proxy.Close)

	var waitGroup sync.WaitGroup
	failures := make(chan string, 1000)
	for range 8 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for range 50 {
				response, err := http.Get(proxy.URL + "/")
				if err != nil {
					failures <- err.Error()
					continue
				}
				if response.StatusCode != http.StatusOK {
					failures <- response.Status
				}
				_ = response.Body.Close()
			}
		}()
	}
	for i := range 200 {
		address := v1
		if i%2 == 0 {
			address = v2
		}
		require.NoError(t, server.SetTable(tableFor(address)))
	}
	waitGroup.Wait()
	close(failures)
	for failure := range failures {
		t.Errorf("request failed during table swap: %s", failure)
	}
}

func TestDecodeTableRejectsInvalidJSON(t *testing.T) {
	_, err := DecodeTable("{")
	assert.Error(t, err)

	server := NewServer(logr.Discard())
	err = server.SetTable(&Table{Routes: []Route{{Protocol: "http", Path: PathMatch{Type: PathMatchRegularExpression, Value: "("}}}})
	assert.Error(t, err, "an invalid regular expression must be rejected")
	assert.False(t, server.Ready())
}
