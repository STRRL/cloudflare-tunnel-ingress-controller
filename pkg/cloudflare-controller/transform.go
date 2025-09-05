package cloudflarecontroller

import (
	"context"
	"strings"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/cloudflare/cloudflare-go/v6"
	"github.com/cloudflare/cloudflare-go/v6/zero_trust"
	"github.com/pkg/errors"
)

func fromExposureToCloudflareIngress(ctx context.Context, exposure exposure.Exposure) (zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress, error) {
	if exposure.IsDeleted {
		return zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress{}, errors.Errorf("exposure %s is deleted, should not generate cloudflare ingress for it", exposure.Hostname)
	}

	result := zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress{
		Hostname: cloudflare.F(exposure.Hostname),
		Service:  cloudflare.F(exposure.ServiceTarget),
	}

	if exposure.PathPrefix != "" {
		result.Path = cloudflare.F(exposure.PathPrefix)
	}

	var originRequest zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngressOriginRequest
	needOriginRequest := false

	if exposure.HTTPHostHeader != nil {
		originRequest.HTTPHostHeader = cloudflare.F(*exposure.HTTPHostHeader)
		needOriginRequest = true
	}

	if strings.HasPrefix(exposure.ServiceTarget, "https://") {
		if exposure.OriginServerName != nil {
			originRequest.OriginServerName = cloudflare.F(*exposure.OriginServerName)
			needOriginRequest = true
		}
		if exposure.ProxySSLVerifyEnabled == nil {
			originRequest.NoTLSVerify = cloudflare.F(true)
			needOriginRequest = true
		} else {
			originRequest.NoTLSVerify = cloudflare.F(!*exposure.ProxySSLVerifyEnabled)
			needOriginRequest = true
		}
	}

	if needOriginRequest {
		result.OriginRequest = cloudflare.F(originRequest)
	}

	return result, nil
}
