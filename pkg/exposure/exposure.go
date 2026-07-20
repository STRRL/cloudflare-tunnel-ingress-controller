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
	// DisableDNSManagement, when true, makes the controller skip Cloudflare DNS
	// record (CNAME/TXT) management for this exposure while still configuring the
	// tunnel ingress rule. DNS can then be delegated to an external system, e.g.
	// external-dns or a Cloudflare Load Balancer targeting the tunnel directly.
	DisableDNSManagement bool
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
