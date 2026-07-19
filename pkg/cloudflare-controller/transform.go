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

	if exposure.HTTPHostHeader != nil {
		if result.OriginRequest == nil {
			result.OriginRequest = &cloudflare.OriginRequestConfig{}
		}
		result.OriginRequest.HTTPHostHeader = exposure.HTTPHostHeader
	}

	if strings.HasPrefix(exposure.ServiceTarget, "https://") {
		if result.OriginRequest == nil {
			result.OriginRequest = &cloudflare.OriginRequestConfig{}
		}
		result.OriginRequest.OriginServerName = exposure.OriginServerName
		if exposure.ProxySSLVerifyEnabled == nil {
			result.OriginRequest.NoTLSVerify = ptr.To(true)
		} else {
			result.OriginRequest.NoTLSVerify = ptr.To(!*exposure.ProxySSLVerifyEnabled)
		}
	}

	return &result, nil
}
