package controller

// AnnotationProxySSLVerify is the annotation key for proxy-ssl-verify, available values: "on" or "off", default "off".
const AnnotationProxySSLVerify = "cloudflare-tunnel-ingress-controller.strrl.dev/proxy-ssl-verify"
const AnnotationProxySSLVerifyOn = "on"
const AnnotationProxySSLVerifyOff = "off"

// AnnotationBackendProtocol is the annotation key for proxy-backend-protocol, default "http".
const AnnotationBackendProtocol = "cloudflare-tunnel-ingress-controller.strrl.dev/backend-protocol"

// AnnotationHTTPHostHeader is to set the HTTP Host header for the local webserver.
const AnnotationHTTPHostHeader = "cloudflare-tunnel-ingress-controller.strrl.dev/http-host-header"

// AnnotationOriginServerName is the hostname on the origin server certificate.
const AnnotationOriginServerName = "cloudflare-tunnel-ingress-controller.strrl.dev/origin-server-name"

// AnnotationAllowedAccessGroup is a comma-separated list of Cloudflare Access Group IDs to allow.
const AnnotationAllowedAccessGroup = "cloudflare-tunnel-ingress-controller.strrl.dev/cloudflare-access-allowed-group"

// AnnotationDeniedAccessGroup is a comma-separated list of Cloudflare Access Group IDs to deny.
const AnnotationDeniedAccessGroup = "cloudflare-tunnel-ingress-controller.strrl.dev/cloudflare-access-denied-group"

// AnnotationAccessBypass when set to "true", creates a bypass Access Application for the hostname.
const AnnotationAccessBypass = "cloudflare-tunnel-ingress-controller.strrl.dev/cloudflare-access-bypass"

// AnnotationAccessSessionDuration sets the session duration for the Access Application (e.g. "1h", "24h").
const AnnotationAccessSessionDuration = "cloudflare-tunnel-ingress-controller.strrl.dev/cloudflare-access-session-duration"

// AnnotationAccessAutoRedirect when "true", skips the IdP selection page and redirects directly to the provider.
const AnnotationAccessAutoRedirect = "cloudflare-tunnel-ingress-controller.strrl.dev/cloudflare-access-auto-redirect"
