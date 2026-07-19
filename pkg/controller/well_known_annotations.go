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

// AnnotationDisableDNSManagement disables Cloudflare DNS record (CNAME/TXT) management
// for this ingress, while still configuring the tunnel ingress rule. This allows DNS to be
// delegated to an external system, e.g. external-dns or a Cloudflare Load Balancer that
// targets the tunnel directly. Available values: "true" or "false", default "false".
const AnnotationDisableDNSManagement = "cloudflare-tunnel-ingress-controller.strrl.dev/disable-dns-management"
const AnnotationDisableDNSManagementTrue = "true"
const AnnotationDisableDNSManagementFalse = "false"
