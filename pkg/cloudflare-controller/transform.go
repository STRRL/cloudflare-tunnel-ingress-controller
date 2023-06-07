package cloudflarecontroller

import (
	"context"
	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/cloudflare/cloudflare-go"
)

func fromExposureToCloudflareIngress(ctx context.Context, exposure exposure.Exposure) (cloudflare.UnvalidatedIngressRule, error) {
	return cloudflare.UnvalidatedIngressRule{
		Hostname: exposure.Hostname,
		Path:     exposure.PathPrefix,
		Service:  exposure.ServiceTarget,
	}, nil
}
