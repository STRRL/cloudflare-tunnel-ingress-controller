package cloudflarecontroller

import (
	"context"
	"strings"

	"github.com/cloudflare/cloudflare-go"
	"github.com/oliverbaehler/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/pkg/errors"
)

func fromExposureToCloudflareIngress(ctx context.Context, exposure exposure.Exposure) (*cloudflare.UnvalidatedIngressRule, error) {
	if exposure.IsDeleted {
		return nil, errors.Errorf("exposure %s is deleted, should not generate cloudflare ingress for it", exposure.Hostname)
	}

	var originRequest = &cloudflare.OriginRequestConfig{}
	result := cloudflare.UnvalidatedIngressRule{
		Hostname: exposure.Hostname,
		Path:     exposure.PathPrefix,
		Service:  exposure.ServiceTarget,
	}

	// Hostname
	originRequest.HTTPHostHeader = &exposure.Config.HttpHostHeader

	// TLS Verification
	if strings.HasPrefix(exposure.ServiceTarget, "https://") {
		if exposure.Config.ProxySSLVerifyEnabled == nil {
			originRequest.NoTLSVerify = boolPointer(true)
		} else {
			originRequest.NoTLSVerify = boolPointer(!*exposure.Config.ProxySSLVerifyEnabled)
		}
	}

	// TLS Timeout
	if exposure.Config.TLSTimeout != nil {
		originRequest.TLSTimeout = exposure.Config.TLSTimeout
	}

	if exposure.Config.ConnectTimeout != nil {
		originRequest.ConnectTimeout = exposure.Config.ConnectTimeout
	}

	if exposure.Config.DisableChunkedEncoding != nil {
		originRequest.DisableChunkedEncoding = exposure.Config.DisableChunkedEncoding
	}

	if exposure.Config.HTTP2Origin != nil {
		originRequest.Http2Origin = exposure.Config.HTTP2Origin
	}

	if exposure.Config.KeepAliveConnections != nil {
		originRequest.KeepAliveConnections = exposure.Config.KeepAliveConnections
	}

	if exposure.Config.KeepAliveTimeout != nil {
		originRequest.KeepAliveTimeout = exposure.Config.KeepAliveTimeout
	}

	// Assign OriginRequest
	result.OriginRequest = originRequest

	return &result, nil
}
