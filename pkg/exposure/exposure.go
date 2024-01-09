package exposure

import "time"

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
	// OriginServerName is the hostname that cloudflared should expect from your origin server certificate. If null, the expected hostname is the service URL, for example localhost if the service is https://localhost:443.
	OriginServerName *string
	// CAPool is the path to the certificate authority (CA) for the certificate of your origin. This option should be used only if your certificate is not signed by Cloudflare.
	CAPool *string
	// When false, TLS verification is performed on the certificate presented by your origin.
	// When true, TLS verification is disabled. This will allow any certificate from the origin to be accepted.
	NoTLSVerify *bool
	// TLSTimeout is the timeout for completing a TLS handshake to your origin server, if you have chosen to connect Tunnel to an HTTPS server.
	TLSTimeout *time.Duration
}
