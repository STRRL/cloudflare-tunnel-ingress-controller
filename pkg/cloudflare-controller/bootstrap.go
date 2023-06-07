package cloudflarecontroller

import (
	"context"
	"github.com/cloudflare/cloudflare-go"
	"github.com/pkg/errors"
)

func BootstrapTunnelClientWithTunnelName(ctx context.Context, cfClient *cloudflare.API, accountId string, tunnelName string) (*TunnelClient, error) {
	tunnelId, err := GetTunnelIdFromTunnelName(ctx, cfClient, tunnelName, accountId)
	if err != nil {
		return nil, errors.Wrapf(err, "get tunnel id from tunnel name %s", tunnelName)
	}
	return NewTunnelClient(cfClient, accountId, tunnelId), nil
}

func GetTunnelIdFromTunnelName(ctx context.Context, cfClient *cloudflare.API, tunnelName string, accountId string) (string, error) {
	// TODO: resolve pagination
	tunnels, _, err := cfClient.ListTunnels(ctx, cloudflare.ResourceIdentifier(accountId), cloudflare.TunnelListParams{
		IsDeleted: boolPointer(false),
	})
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
