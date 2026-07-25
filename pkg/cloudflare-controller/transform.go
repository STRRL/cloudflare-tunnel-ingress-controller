package cloudflarecontroller

import (
	"context"
	"reflect"
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

	result.OriginRequest = fromExposureToOriginRequest(exposure)

	return &result, nil
}

// fromExposureToOriginRequest assembles the originRequest settings of a rule,
// it returns nil when the exposure carries no origin request related setting.
func fromExposureToOriginRequest(exposure exposure.Exposure) *cloudflare.OriginRequestConfig {
	config := cloudflare.OriginRequestConfig{}

	if exposure.HTTPHostHeader != nil {
		config.HTTPHostHeader = exposure.HTTPHostHeader
	}

	if strings.HasPrefix(exposure.ServiceTarget, "https://") {
		config.OriginServerName = exposure.OriginServerName
		if exposure.ProxySSLVerifyEnabled == nil {
			config.NoTLSVerify = ptr.To(true)
		} else {
			config.NoTLSVerify = ptr.To(!*exposure.ProxySSLVerifyEnabled)
		}
	}

	// direct no-tls-verify control takes precedence over the legacy
	// proxy-ssl-verify derived value above
	if exposure.NoTLSVerify != nil {
		config.NoTLSVerify = exposure.NoTLSVerify
	}

	if exposure.ConnectTimeout != nil {
		config.ConnectTimeout = &cloudflare.TunnelDuration{Duration: *exposure.ConnectTimeout}
	}
	if exposure.TLSTimeout != nil {
		config.TLSTimeout = &cloudflare.TunnelDuration{Duration: *exposure.TLSTimeout}
	}
	if exposure.TCPKeepAlive != nil {
		config.TCPKeepAlive = &cloudflare.TunnelDuration{Duration: *exposure.TCPKeepAlive}
	}
	if exposure.KeepAliveTimeout != nil {
		config.KeepAliveTimeout = &cloudflare.TunnelDuration{Duration: *exposure.KeepAliveTimeout}
	}
	if exposure.NoHappyEyeballs != nil {
		config.NoHappyEyeballs = exposure.NoHappyEyeballs
	}
	if exposure.KeepAliveConnections != nil {
		config.KeepAliveConnections = exposure.KeepAliveConnections
	}
	if exposure.DisableChunkedEncoding != nil {
		config.DisableChunkedEncoding = exposure.DisableChunkedEncoding
	}
	if exposure.HTTP2Origin != nil {
		config.Http2Origin = exposure.HTTP2Origin
	}

	if reflect.DeepEqual(config, cloudflare.OriginRequestConfig{}) {
		return nil
	}
	return &config
}
