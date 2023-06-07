package cloudflarecontroller

import (
	"context"
	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
)

type TunnelClient struct {
}

func (receiver *TunnelClient) PutExposures(ctx context.Context, exposures []exposure.Exposure) error {
	panic("implement me")
}
