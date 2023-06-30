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

	result := cloudflare.UnvalidatedIngressRule{
		Hostname: exposure.Hostname,
		Path:     exposure.PathPrefix,
		Service:  exposure.ServiceTarget,
	}

	if strings.HasPrefix(exposure.ServiceTarget, "https://") {
		result.OriginRequest = &cloudflare.OriginRequestConfig{}
		if exposure.ProxySSLVerifyEnabled == nil {
			result.OriginRequest.NoTLSVerify = boolPointer(true)
		} else {
			result.OriginRequest.NoTLSVerify = boolPointer(!*exposure.ProxySSLVerifyEnabled)
		}
	}

	return &result, nil
}
