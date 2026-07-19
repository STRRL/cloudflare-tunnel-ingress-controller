package e2e

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cloudflare/cloudflare-go"
)

func newCloudflareAPI() (*cloudflare.API, error) {
	return cloudflare.NewWithAPIToken(os.Getenv("CLOUDFLARE_API_TOKEN"))
}

// findZoneID resolves the zone covering the hostname by picking the longest
// zone name that is a suffix of the hostname.
func findZoneID(ctx context.Context, api *cloudflare.API, hostname string) (string, error) {
	zones, err := api.ListZones(ctx)
	if err != nil {
		return "", fmt.Errorf("list zones: %w", err)
	}

	var bestID string
	var bestName string
	for _, zone := range zones {
		if hostname != zone.Name && !strings.HasSuffix(hostname, "."+zone.Name) {
			continue
		}
		if len(zone.Name) > len(bestName) {
			bestID = zone.ID
			bestName = zone.Name
		}
	}
	if bestID == "" {
		return "", fmt.Errorf("no zone found for hostname %s", hostname)
	}
	return bestID, nil
}

func dnsRecordExists(ctx context.Context, api *cloudflare.API, zoneID string, recordType string, name string) (bool, error) {
	records, _, err := api.ListDNSRecords(ctx, cloudflare.ResourceIdentifier(zoneID), cloudflare.ListDNSRecordsParams{
		Type: recordType,
		Name: name,
	})
	if err != nil {
		return false, fmt.Errorf("list %s records for %s: %w", recordType, name, err)
	}
	return len(records) > 0, nil
}
