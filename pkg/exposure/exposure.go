package exposure

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
	// AllowedAccessGroupIDs is the list of Cloudflare Access Group IDs to allow.
	// When non-empty, a Cloudflare Access Application is created for the hostname.
	AllowedAccessGroupIDs []string
	// DeniedAccessGroupIDs is the list of Cloudflare Access Group IDs to deny.
	// Creates a higher-precedence deny policy on the Access Application.
	DeniedAccessGroupIDs []string
	// AccessBypass when true creates a bypass Access Application (no auth required).
	AccessBypass bool
	// AccessSessionDuration overrides the default session duration (e.g. "1h", "24h").
	AccessSessionDuration string
	// AccessAutoRedirect when non-nil, controls whether to skip the IdP selection page.
	AccessAutoRedirect *bool
}
