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

// The annotations below map to cloudflared originRequest settings applied to
// every rule generated from the ingress. See
// https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/configure-tunnels/origin-parameters/

// AnnotationConnectTimeout is the timeout for establishing a new TCP connection to the origin,
// as a Go duration string in whole seconds, eg. "30s".
const AnnotationConnectTimeout = "cloudflare-tunnel-ingress-controller.strrl.dev/connect-timeout"

// AnnotationTLSTimeout is the timeout for completing a TLS handshake with the origin,
// as a Go duration string in whole seconds, eg. "10s".
const AnnotationTLSTimeout = "cloudflare-tunnel-ingress-controller.strrl.dev/tls-timeout"

// AnnotationTCPKeepAlive is the TCP keepalive interval for connections to the origin,
// as a Go duration string in whole seconds, eg. "30s".
const AnnotationTCPKeepAlive = "cloudflare-tunnel-ingress-controller.strrl.dev/tcp-keepalive"

// AnnotationNoHappyEyeballs disables the IPv4/IPv6 fallback when connecting to the origin,
// available values: "true" or "false".
const AnnotationNoHappyEyeballs = "cloudflare-tunnel-ingress-controller.strrl.dev/no-happy-eyeballs"

// AnnotationKeepAliveConnections is the maximum keepalive connection pool size towards the origin,
// as an integer, eg. "100".
const AnnotationKeepAliveConnections = "cloudflare-tunnel-ingress-controller.strrl.dev/keepalive-connections"

// AnnotationKeepAliveTimeout is the timeout for closing idle connections to the origin,
// as a Go duration string in whole seconds, eg. "90s".
const AnnotationKeepAliveTimeout = "cloudflare-tunnel-ingress-controller.strrl.dev/keepalive-timeout"

// AnnotationNoTLSVerify disables TLS certificate verification of the origin, available
// values: "true" or "false". Mutually exclusive with proxy-ssl-verify, which expresses
// the same setting inverted.
const AnnotationNoTLSVerify = "cloudflare-tunnel-ingress-controller.strrl.dev/no-tls-verify"

// AnnotationDisableChunkedEncoding disables chunked transfer encoding towards the origin,
// useful for WSGI servers, available values: "true" or "false".
const AnnotationDisableChunkedEncoding = "cloudflare-tunnel-ingress-controller.strrl.dev/disable-chunked-encoding"

// AnnotationHTTP2Origin connects to the origin with HTTP/2, available values: "true" or "false".
const AnnotationHTTP2Origin = "cloudflare-tunnel-ingress-controller.strrl.dev/http2-origin"
