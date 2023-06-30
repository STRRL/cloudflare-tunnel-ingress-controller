package cloudflarecontroller

import (
	"context"
	"crypto/rand"
	"fmt"

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
	return NewTunnelClient(logger, cfClient, accountId, tunnelId, tunnelName), nil
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

	// create tunnel if not found
	logger.V(3).Info("tunnel not found, create tunnel", "account-id", accountId, "tunnel-name", tunnelName)
	randomSecret := make([]byte, 64)
	_, err = rand.Read(randomSecret)
	if err != nil {
		return "", errors.Wrap(err, "generate random secret")
	}

	hexSecret := fmt.Sprintf("%x", randomSecret)
	newTunnel, err := cfClient.CreateTunnel(ctx, cloudflare.ResourceIdentifier(accountId), cloudflare.TunnelCreateParams{
		Name:      tunnelName,
		Secret:    hexSecret,
		ConfigSrc: "cloudflare",
	})
	if err != nil {
		return "", errors.Wrapf(err, "create tunnel %s", tunnelName)
	}

	return newTunnel.ID, nil
}

func boolPointer(b bool) *bool {
	return &b
}
