package cloudflarecontroller

import (
	"context"
	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/cloudflare/cloudflare-go"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

type TunnelClient struct {
	logger    logr.Logger
	cfClient  *cloudflare.API
	accountId string
	tunnelId  string
}

func NewTunnelClient(logger logr.Logger, cfClient *cloudflare.API, accountId string, tunnelId string) *TunnelClient {
	return &TunnelClient{logger: logger, cfClient: cfClient, accountId: accountId, tunnelId: tunnelId}
}

func (t *TunnelClient) PutExposures(ctx context.Context, exposures []exposure.Exposure) error {
	var ingressRules []cloudflare.UnvalidatedIngressRule

	for _, item := range exposures {
		ingress, err := fromExposureToCloudflareIngress(ctx, item)
		if err != nil {
			return errors.Wrapf(err, "transform to cloudflare ingress")
		}
		ingressRules = append(ingressRules, ingress)
	}

	_, err := t.cfClient.UpdateTunnelConfiguration(ctx,
		cloudflare.ResourceIdentifier(t.accountId),
		cloudflare.TunnelConfigurationParams{
			TunnelID: t.tunnelId,
			Config: cloudflare.TunnelConfiguration{
				Ingress: []cloudflare.UnvalidatedIngressRule{},
			},
		},
	)

	if err != nil {
		return errors.Wrap(err, "update cloudflare tunnel config")
	}
	return nil
}
