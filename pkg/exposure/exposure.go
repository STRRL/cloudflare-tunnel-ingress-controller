package exposure

import "time"

// Exposure is the minimal information for exposing a service.
type Exposure struct {
	// Hostname is the domain name to expose the service, eg. hello.strrl.dev
	Hostname string
	// ServiceTarget is the url of the service to expose, eg. http://my-service.default.svc.cluster.local:9117
	ServiceTarget string
	// PathPrefix is the path prefix to expose the service, eg. /hello
	PathPrefix string
	// IsDeleted is the flag to indicate if the exposure is deleted.
	IsDeleted bool
	// ProxySSLVerifyEnabled is the flag to indicate if the exposure should skip TLS verification.
	ProxySSLVerifyEnabled *bool
	// HTTPHostHeader is to set the HTTP Host header for the local webserver.
	HTTPHostHeader *string
	// OriginServerName is the hostname on the origin server certificate.
	OriginServerName *string
	// DisableDNSManagement, when true, makes the controller skip Cloudflare DNS
	// record (CNAME/TXT) management for this exposure while still configuring the
	// tunnel ingress rule. DNS can then be delegated to an external system, e.g.
	// external-dns or a Cloudflare Load Balancer targeting the tunnel directly.
	DisableDNSManagement bool

	// The fields below map to cloudflared originRequest settings, nil means
	// the cloudflared default applies.

	// ConnectTimeout is the timeout for establishing a new TCP connection to the origin.
	ConnectTimeout *time.Duration
	// TLSTimeout is the timeout for completing a TLS handshake with the origin.
	TLSTimeout *time.Duration
	// TCPKeepAlive is the TCP keepalive interval for connections to the origin.
	TCPKeepAlive *time.Duration
	// NoHappyEyeballs disables the IPv4/IPv6 fallback when connecting to the origin.
	NoHappyEyeballs *bool
	// KeepAliveConnections is the maximum keepalive connection pool size towards the origin.
	KeepAliveConnections *int
	// KeepAliveTimeout is the timeout for closing idle connections to the origin.
	KeepAliveTimeout *time.Duration
	// NoTLSVerify disables TLS certificate verification of the origin, it takes
	// precedence over ProxySSLVerifyEnabled when set.
	NoTLSVerify *bool
	// DisableChunkedEncoding disables chunked transfer encoding towards the origin.
	DisableChunkedEncoding *bool
	// HTTP2Origin connects to the origin with HTTP/2.
	HTTP2Origin *bool
}

// Active returns the exposures that are not marked as deleted, preserving order.
func Active(exposures []Exposure) []Exposure {
	var active []Exposure
	for _, item := range exposures {
		if !item.IsDeleted {
			active = append(active, item)
		}
	}
	return active
}
