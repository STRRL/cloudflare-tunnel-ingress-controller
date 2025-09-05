package cloudflarecontroller

import (
	"context"
	"crypto/rand"
	"fmt"

	"github.com/cloudflare/cloudflare-go/v6"
	"github.com/cloudflare/cloudflare-go/v6/zero_trust"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

func BootstrapTunnelClientWithTunnelName(ctx context.Context, logger logr.Logger, cfClient *cloudflare.Client, accountId string, tunnelName string) (*TunnelClient, error) {
	logger.V(3).Info("fetch tunnel id with tunnel name", "account-id", accountId, "tunnel-name", tunnelName)
	tunnelId, err := GetTunnelIdFromTunnelName(ctx, logger, cfClient, tunnelName, accountId)
	if err != nil {
		return nil, errors.Wrapf(err, "get tunnel id from tunnel name %s", tunnelName)
	}
	logger.V(3).Info("tunnel id fetched", "tunnel-id", tunnelId, "tunnel-name", tunnelName, "account-id", accountId)
	return NewTunnelClient(logger, cfClient, accountId, tunnelId, tunnelName), nil
}

func GetTunnelIdFromTunnelName(ctx context.Context, logger logr.Logger, cfClient *cloudflare.Client, tunnelName string, accountId string) (string, error) {
	logger.V(3).Info("list cloudflare tunnels", "account-id", accountId)
	// Use auto-paging to get all tunnels
	var tunnels []zero_trust.TunnelListResponse
	pager := cfClient.ZeroTrust.Tunnels.ListAutoPaging(ctx, zero_trust.TunnelListParams{
		AccountID: cloudflare.F(accountId),
		IsDeleted: cloudflare.F(false),
	})

	for pager.Next() {
		tunnel := pager.Current()
		tunnels = append(tunnels, tunnel)
	}

	if pager.Err() != nil {
		return "", errors.Wrap(pager.Err(), "list cloudflare tunnels")
	}

	logger.V(3).Info("list cloudflare tunnels complete", "account-id", accountId, "tunnel-count", len(tunnels))

	for _, tunnel := range tunnels {
		if tunnel.Name == tunnelName {
			return tunnel.ID, nil
		}
	}

	// create tunnel if not found
	logger.V(3).Info("tunnel not found, create tunnel", "account-id", accountId, "tunnel-name", tunnelName)
	randomSecret := make([]byte, 64)
	_, err := rand.Read(randomSecret)
	if err != nil {
		return "", errors.Wrap(err, "generate random secret")
	}

	hexSecret := fmt.Sprintf("%x", randomSecret)
	newTunnelResponse, err := cfClient.ZeroTrust.Tunnels.Cloudflared.New(ctx, zero_trust.TunnelCloudflaredNewParams{
		AccountID:    cloudflare.F(accountId),
		Name:         cloudflare.F(tunnelName),
		TunnelSecret: cloudflare.F(hexSecret),
		ConfigSrc:    cloudflare.F(zero_trust.TunnelCloudflaredNewParamsConfigSrcCloudflare),
	})
	if err != nil {
		return "", errors.Wrapf(err, "create tunnel %s", tunnelName)
	}

	return newTunnelResponse.ID, nil
}

func boolPointer(b bool) *bool {
	return &b
}
