package cloudflarecontroller

import (
	"context"

	"github.com/cloudflare/cloudflare-go"
	"github.com/oliverbaehler/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/pkg/errors"
)

func fromExposureToCloudflareIngress(ctx context.Context, exposure exposure.Exposure) (*cloudflare.UnvalidatedIngressRule, error) {
	if exposure.IsDeleted {
		return nil, errors.Errorf("exposure %s is deleted, should not generate cloudflare ingress for it", exposure.Hostname)
	}

	result := cloudflare.UnvalidatedIngressRule{
		Hostname:      exposure.Hostname,
		Path:          exposure.PathPrefix,
		Service:       exposure.ServiceTarget,
		OriginRequest: &exposure.OriginRequest,
	}

	return &result, nil
}
