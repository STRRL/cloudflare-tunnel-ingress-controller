package controller

const AnnotationBase = "cloudflare-tunnel-ingress-controller.strrl.dev"

const AnnotationValueBooleanTrue = "on"
const AnnotationValueBooleanFalse = "off"

// AnnotationProxySSLVerify is the annotation key for proxy-ssl-verify, available values: "on" or "off", default "off".
var AnnotationProxySSLVerify = AnnotationBase + "/proxy-ssl-verify"

// AnnotationBackendProtocol is the annotation key for proxy-backend-protocol, default "http".
var AnnotationBackendProtocol = AnnotationBase + "/backend-protocol"

// AnnotationBackendProtocol is the annotation key for Timeout for establishing a new TCP connection to your origin server. This excludes the time taken to establish TLS, which is controlled by tlsTimeout.
var AnnotationConnectionTimeount = AnnotationBase + "/connection-timeout"

var AnnotationChunkedEncoding = AnnotationBase + "/chunked-encoding"

var AnnotationHTTP20Origin = AnnotationBase + "/http2Origin"

var AnnotationTLSTimeout = AnnotationBase + "/tls-timeout"

var AnnotationTCPKeepAliveInterval = AnnotationBase + "/tcp-keep-alive"

// AnnotationTCPKeepAliveConnections is the annotation key for Maximum number of idle keepalive connections between Tunnel and your origin. This does not restrict the total number of concurrent connections.
var AnnotationTCPKeepAliveConnections = AnnotationBase + "/tcp-keep-alive-connections"

// AnnotationTCPKeepAliveTimeout is the annotation key for the timeout after which a TCP keepalive packet is sent on a connection between Tunnel and the origin server.
var AnnotationTCPKeepAliveTimeout = AnnotationBase + "/tcp-keep-alive-timeout"

var AnnotationHappyEyeballs = AnnotationBase + "/happy-eyeballs"
