package controller

// AnnotationProxySSLVerify is the annotation key for proxy-ssl-verify, available values: "on" or "off", default "off".
const AnnotationProxySSLVerify = "cloudflare-tunnel-ingress-controller.strrl.dev/proxy-ssl-verify"
const AnnotationProxySSLVerifyOn = "on"
const AnnotationProxySSLVerifyOff = "off"

// AnnotationBackendProtocol is the annotation key for proxy-backend-protocol, default "http".
const AnnotationBackendProtocol = "cloudflare-tunnel-ingress-controller.strrl.dev/backend-protocol"

// AnnotationOriginServerName is the annotation for the hostname that cloudflared should expect from your origin server certificate. If null, the expected hostname is the service URL, for example localhost if the service is https://localhost:443.
const AnnotationOriginServerName = "cloudflare-tunnel-ingress-controller.strrl.dev/origin-server-name"

// AnnotationCAPool is the annotation for the path to the certificate authority (CA) for the certificate of your origin. This option should be used only if your certificate is not signed by Cloudflare.
const AnnotationCAPool = "cloudflare-tunnel-ingress-controller.strrl.dev/origin-capool"

// AnnotationTLSTimeout is the timeout for completing a TLS handshake to your origin server, if you have chosen to connect Tunnel to an HTTPS server.
const AnnotationTLSTimeout = "cloudflare-tunnel-ingress-controller.strrl.dev/origin-tls-timeout"
