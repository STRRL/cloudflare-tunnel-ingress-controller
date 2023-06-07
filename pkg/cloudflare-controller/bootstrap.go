package cloudflarecontroller

import (
	"context"
	"github.com/cloudflare/cloudflare-go"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

func BootstrapTunnelClientWithTunnelName(ctx context.Context, logger logr.Logger, cfClient *cloudflare.API, accountId string, tunnelName string) (*TunnelClient, error) {
	logger.V(3).Info("fetch tunnel id with tunnel name", "account-id", accountId, "tunnel-name", tunnelName)
	tunnelId, err := GetTunnelIdFromTunnelName(ctx, logger, cfClient, tunnelName, accountId)
	if err != nil {
		return nil, errors.Wrapf(err, "get tunnel id from tunnel name %s", tunnelName)
	}
	logger.V(3).Info("tunnel id fetched", "tunnel-id", tunnelId, "tunnel-name", tunnelName, "account-id", accountId)
	return NewTunnelClient(logger, cfClient, accountId, tunnelId), nil
}

func GetTunnelIdFromTunnelName(ctx context.Context, logger logr.Logger, cfClient *cloudflare.API, tunnelName string, accountId string) (string, error) {
	logger.V(3).Info("list cloudflare tunnels", "account-id", accountId)
	tunnels, _, err := cfClient.ListTunnels(ctx, cloudflare.ResourceIdentifier(accountId), cloudflare.TunnelListParams{
		IsDeleted: boolPointer(false),
		// FIXME: that's a workaround for https://github.com/cloudflare/cloudflare-go/issues/1247
		ResultInfo: cloudflare.ResultInfo{
			Page:    1,
			PerPage: 1000,
		},
	})
	logger.V(3).Info("list cloudflare tunnels complete", "account-id", accountId, "tunnels", tunnels)

	if err != nil {
		return "", errors.Wrap(err, "list cloudflare tunnels")
	}
	for _, tunnel := range tunnels {
		if tunnel.Name == tunnelName {
			return tunnel.ID, nil
		}
	}
	return "", errors.Errorf("tunnel %s not found", tunnelName)
}

func boolPointer(b bool) *bool {
	return &b
}
