// Package conformance runs the Gateway API conformance suite against the
// controller through the real path: test runner, Cloudflare edge, tunnel,
// cloudflared, per Gateway proxy, backend. Run it with
// test/conformance/conformance.sh, see the Makefile conformance targets.
package conformance

import (
	"net"
	"os"
	"testing"

	"sigs.k8s.io/gateway-api/conformance"
	confv1 "sigs.k8s.io/gateway-api/conformance/apis/v1"
	"sigs.k8s.io/gateway-api/conformance/utils/flags"
	"sigs.k8s.io/gateway-api/conformance/utils/roundtripper"
	"sigs.k8s.io/gateway-api/conformance/utils/suite"
	"sigs.k8s.io/gateway-api/pkg/features"
)

// edgeSkipTests send requests with a Host outside the test zone (example.com,
// bar.com, ...) or need TLS with the test's own certificate. The Cloudflare
// edge routes by Host and serves its own certificate, so these requests can
// never reach the tunnel. Their request cases are covered by unit tests in
// pkg/gatewayproxy and pkg/controller.
var edgeSkipTests = []string{
	"HTTPRouteHTTPSListener",
	"HTTPRouteHostnameIntersection",
	"HTTPRouteListenerHostnameMatching",
	"HTTPRouteMatchingAcrossRoutes",
}

func TestConformance(t *testing.T) {
	baseDomain := os.Getenv("E2E_BASE_DOMAIN")
	if baseDomain == "" {
		t.Skip("E2E_BASE_DOMAIN is not set, run through test/conformance/conformance.sh")
	}

	options := conformance.DefaultOptions(t)
	if options.GatewayClassName == flags.DefaultGatewayClassName {
		options.GatewayClassName = "cloudflare-tunnel"
	}
	options.ConformanceProfiles = []suite.ConformanceProfileName{suite.GatewayHTTPConformanceProfileName}
	options.SupportedFeatures = []features.FeatureName{
		features.SupportGateway,
		features.SupportHTTPRoute,
		features.SupportReferenceGrant,
	}
	options.SkipTests = append(options.SkipTests, edgeSkipTests...)

	dialer, err := newAuthoritativeDialer(baseDomain, os.Getenv("CONFORMANCE_RESOLVER"))
	if err != nil {
		t.Fatalf("create dialer: %v", err)
	}
	options.RoundTripper = &edgeRoundTripper{DefaultRoundTripper: roundtripper.DefaultRoundTripper{
		Debug:             options.Debug,
		TimeoutConfig:     options.TimeoutConfig,
		CustomDialContext: dialer,
	}}

	options.Implementation = confv1.Implementation{
		Organization: "STRRL",
		Project:      "cloudflare-tunnel-ingress-controller",
		URL:          "https://github.com/STRRL/cloudflare-tunnel-ingress-controller",
		Version:      "dev",
		Contact:      []string{"@STRRL"},
	}

	conformance.RunConformanceWithOptions(t, options)
}

// edgeRoundTripper sends the suite's plain http requests to the Cloudflare
// edge over https. A zone with Always Use HTTPS answers http with its own
// redirect before the tunnel, so https is the scheme every client of the
// zone ends up with. Gateway addresses are one level below the zone, the
// Universal SSL certificate covers them. Everything else stays as the suite
// sent it, and the request still travels edge, tunnel, cloudflared, proxy,
// backend.
type edgeRoundTripper struct {
	roundtripper.DefaultRoundTripper
}

func (e *edgeRoundTripper) CaptureRoundTrip(request roundtripper.Request) (*roundtripper.CapturedRequest, *roundtripper.CapturedResponse, error) {
	if request.URL.Scheme == "http" {
		request.URL.Scheme = "https"
		if host, port, err := net.SplitHostPort(request.URL.Host); err == nil && port == "80" {
			request.URL.Host = host
		}
	}
	return e.DefaultRoundTripper.CaptureRoundTrip(request)
}
