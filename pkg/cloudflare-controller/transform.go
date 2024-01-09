package cloudflarecontroller

import (
	"context"
	"strings"
	
	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/cloudflare/cloudflare-go"
	"github.com/pkg/errors"
)

func fromExposureToCloudflareIngress(ctx context.Context, exposure exposure.Exposure) (*cloudflare.UnvalidatedIngressRule, error) {
	if exposure.IsDeleted {
		return nil, errors.Errorf("exposure %s is deleted, should not generate cloudflare ingress for it", exposure.Hostname)
	}

	var originRequest *cloudflare.OriginRequestConfig = nil

	if exposure.OriginServerName != nil {
		if originRequest == nil {
			originRequest = &cloudflare.OriginRequestConfig{}
		}

		originRequest.OriginServerName = exposure.OriginServerName
	}

	if exposure.CAPool != nil {
		if originRequest == nil {
			originRequest = &cloudflare.OriginRequestConfig{}
		}

		originRequest.CAPool = exposure.CAPool
	}

	if exposure.TLSTimeout != nil {
		if originRequest == nil {
			originRequest = &cloudflare.OriginRequestConfig{}
		}

		originRequest.TLSTimeout = &cloudflare.TunnelDuration{
			Duration: *exposure.TLSTimeout,
		}
	}

	if strings.HasPrefix(exposure.ServiceTarget, "https://") {
		if originRequest == nil {
			originRequest = &cloudflare.OriginRequestConfig{}
		}

		if exposure.NoTLSVerify == nil {
			originRequest.NoTLSVerify = boolPointer(true)
		} else {
			originRequest.NoTLSVerify = exposure.NoTLSVerify
		}
	}

	result := cloudflare.UnvalidatedIngressRule{
		Hostname:      exposure.Hostname,
		Path:          exposure.PathPrefix,
		Service:       exposure.ServiceTarget,
		OriginRequest: originRequest,
	}

	return &result, nil
}
