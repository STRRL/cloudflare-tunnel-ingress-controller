package exposure

// Exposure is the minimal information for exposing a service.
type Exposure struct {
	// Hostname is the domain name to expose the service, eg. hello.strrl.dev
	Hostname string
	// ServiceTarget is the url of the service to expose, eg. http://10.109.94.106:9117
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
	// AccessApplicationName is the name of the Access application to create.
	AccessApplicationName *string
	// AccessPolicyAllowedEmails is a list of email addresses to allow access to the application.
	AccessPolicyAllowedEmails []string
}
