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
