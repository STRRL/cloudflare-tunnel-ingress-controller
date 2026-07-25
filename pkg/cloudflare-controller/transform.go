package cloudflarecontroller

import (
	"context"
	"strings"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/cloudflare/cloudflare-go"
	"github.com/pkg/errors"
	"k8s.io/utils/ptr"
)

func fromExposureToCloudflareIngress(ctx context.Context, exposure exposure.Exposure) (*cloudflare.UnvalidatedIngressRule, error) {
	if exposure.IsDeleted {
		return nil, errors.Errorf("exposure %s is deleted, should not generate cloudflare ingress for it", exposure.Hostname)
	}

	result := cloudflare.UnvalidatedIngressRule{
		Hostname: exposure.Hostname,
		Service:  exposure.ServiceTarget,
	}

	// path based routing only applies to http(s), non http protocols
	// like ssh, rdp or tcp must not carry a path in the tunnel rule
	if strings.HasPrefix(exposure.ServiceTarget, "http://") || strings.HasPrefix(exposure.ServiceTarget, "https://") {
		result.Path = exposure.PathPrefix
	}

	originRequest := func() *cloudflare.OriginRequestConfig {
		if result.OriginRequest == nil {
			result.OriginRequest = &cloudflare.OriginRequestConfig{}
		}
		return result.OriginRequest
	}

	if exposure.HTTPHostHeader != nil {
		originRequest().HTTPHostHeader = exposure.HTTPHostHeader
	}

	if strings.HasPrefix(exposure.ServiceTarget, "https://") {
		originRequest().OriginServerName = exposure.OriginServerName
		if exposure.ProxySSLVerifyEnabled == nil {
			originRequest().NoTLSVerify = ptr.To(true)
		} else {
			originRequest().NoTLSVerify = ptr.To(!*exposure.ProxySSLVerifyEnabled)
		}
	}

	// direct no-tls-verify control takes precedence over the legacy
	// proxy-ssl-verify derived value above
	if exposure.NoTLSVerify != nil {
		originRequest().NoTLSVerify = exposure.NoTLSVerify
	}

	if exposure.ConnectTimeout != nil {
		originRequest().ConnectTimeout = &cloudflare.TunnelDuration{Duration: *exposure.ConnectTimeout}
	}
	if exposure.TLSTimeout != nil {
		originRequest().TLSTimeout = &cloudflare.TunnelDuration{Duration: *exposure.TLSTimeout}
	}
	if exposure.TCPKeepAlive != nil {
		originRequest().TCPKeepAlive = &cloudflare.TunnelDuration{Duration: *exposure.TCPKeepAlive}
	}
	if exposure.KeepAliveTimeout != nil {
		originRequest().KeepAliveTimeout = &cloudflare.TunnelDuration{Duration: *exposure.KeepAliveTimeout}
	}
	if exposure.NoHappyEyeballs != nil {
		originRequest().NoHappyEyeballs = exposure.NoHappyEyeballs
	}
	if exposure.KeepAliveConnections != nil {
		originRequest().KeepAliveConnections = exposure.KeepAliveConnections
	}
	if exposure.DisableChunkedEncoding != nil {
		originRequest().DisableChunkedEncoding = exposure.DisableChunkedEncoding
	}
	if exposure.HTTP2Origin != nil {
		originRequest().Http2Origin = exposure.HTTP2Origin
	}

	return &result, nil
}
