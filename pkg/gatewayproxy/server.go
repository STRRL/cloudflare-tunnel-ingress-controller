package gatewayproxy

import (
	"context"
	"math/rand/v2"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/go-logr/logr"
)

// Server answers requests with the current routing table. The table can be
// replaced at any time without dropping requests in flight.
type Server struct {
	logger  logr.Logger
	current atomic.Pointer[matcher]
	proxy   *httputil.ReverseProxy
}

type forwardKey struct{}

// forward is what Rewrite needs to know about the chosen route and backend.
type forward struct {
	route   *Route
	address string
}

func NewServer(logger logr.Logger) *Server {
	server := &Server{logger: logger}
	server.proxy = &httputil.ReverseProxy{
		Rewrite: func(request *httputil.ProxyRequest) {
			target := request.In.Context().Value(forwardKey{}).(forward)
			request.Out.URL.Scheme = "http"
			request.Out.URL.Host = target.address
			// keep the Host the client sent, backends see the original hostname
			request.Out.Host = request.In.Host
			// backends see the client chain and the scheme the client used
			// at the edge, cloudflared sends it as X-Forwarded-Proto
			request.Out.Header["X-Forwarded-For"] = request.In.Header["X-Forwarded-For"]
			request.SetXForwarded()
			request.Out.Header.Set("X-Forwarded-Proto", requestScheme(request.In))
			applyHeaderModifier(request.Out.Header, target.route.RequestHeaderModifier)
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			logger.Error(err, "forward request", "host", r.Host, "path", r.URL.Path)
			w.WriteHeader(http.StatusBadGateway)
		},
	}
	return server
}

// SetTable replaces the routing table used for new requests.
func (s *Server) SetTable(table *Table) error {
	m, err := newMatcher(table)
	if err != nil {
		return err
	}
	s.current.Store(m)
	return nil
}

// Ready reports whether a routing table has been loaded.
func (s *Server) Ready() bool {
	return s.current.Load() != nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m := s.current.Load()
	if m == nil {
		http.Error(w, "route table not loaded", http.StatusServiceUnavailable)
		return
	}

	route := m.find(r)
	if route == nil {
		http.NotFound(w, r)
		return
	}

	if route.Redirect != nil {
		w.Header().Set("Location", redirectLocation(r, route))
		statusCode := route.Redirect.StatusCode
		if statusCode == 0 {
			statusCode = http.StatusFound
		}
		w.WriteHeader(statusCode)
		return
	}

	backend := pickBackend(route.Backends)
	if backend == nil || backend.Invalid || len(backend.Addresses) == 0 {
		http.Error(w, "no valid backend", http.StatusInternalServerError)
		return
	}
	address := backend.Addresses[rand.IntN(len(backend.Addresses))]

	ctx := context.WithValue(r.Context(), forwardKey{}, forward{route: route, address: address})
	s.proxy.ServeHTTP(w, r.WithContext(ctx))
}

// pickBackend chooses a backend at random, in proportion to the weights.
// It returns nil when no backend has a positive weight.
func pickBackend(backends []Backend) *Backend {
	var total int32
	for _, backend := range backends {
		total += max(backend.Weight, 0)
	}
	if total == 0 {
		return nil
	}
	pick := rand.Int32N(total)
	for i := range backends {
		weight := max(backends[i].Weight, 0)
		if pick < weight {
			return &backends[i]
		}
		pick -= weight
	}
	return nil
}

func applyHeaderModifier(header http.Header, modifier *HeaderModifier) {
	if modifier == nil {
		return
	}
	for _, item := range modifier.Set {
		header.Set(item.Name, item.Value)
	}
	for _, item := range modifier.Add {
		header.Add(item.Name, item.Value)
	}
	for _, name := range modifier.Remove {
		header.Del(name)
	}
}

// requestScheme is the scheme the client used at the Cloudflare edge,
// forwarded as X-Forwarded-Proto. A missing header means plain http.
func requestScheme(r *http.Request) string {
	if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		return "https"
	}
	return "http"
}

// redirectLocation builds the Location header. Without an explicit scheme
// the scheme of the client request is kept. Well known ports are left out,
// the edge only listens on them.
func redirectLocation(r *http.Request, route *Route) string {
	redirect := route.Redirect
	scheme := redirect.Scheme
	if scheme == "" {
		scheme = requestScheme(r)
	}
	host := redirect.Hostname
	if host == "" {
		host = requestHost(r)
	}
	port := redirect.Port
	if (scheme == "http" && port == 80) || (scheme == "https" && port == 443) {
		port = 0
	}
	if port != 0 {
		host = net.JoinHostPort(host, strconv.Itoa(port))
	}
	location := url.URL{
		Scheme:   scheme,
		Host:     host,
		Path:     r.URL.Path,
		RawPath:  r.URL.RawPath,
		RawQuery: r.URL.RawQuery,
	}
	return location.String()
}
