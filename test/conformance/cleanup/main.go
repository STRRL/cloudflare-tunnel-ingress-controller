// Command cleanup removes everything a conformance run leaves in Cloudflare:
// DNS records named *-gwc-<RUN_ID>.<E2E_BASE_DOMAIN> (Gateway addresses and
// their ownership TXT records) and the rules of the run tunnel. It needs no cluster, so it also cleans up after a crashed run. It
// first checks zone settings that break the suite and warns about them.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"k8s.io/utils/ptr"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "cleanup failed: %v\n", err)
		os.Exit(1)
	}
}

func requireEnv(name string) (string, error) {
	value := os.Getenv(name)
	if value == "" {
		return "", fmt.Errorf("environment variable %s is required", name)
	}
	return value, nil
}

func run() error {
	values := map[string]string{}
	for _, name := range []string{"CLOUDFLARE_API_TOKEN", "CLOUDFLARE_ACCOUNT_ID", "CLOUDFLARE_TUNNEL_NAME", "E2E_BASE_DOMAIN", "RUN_ID"} {
		value, err := requireEnv(name)
		if err != nil {
			return err
		}
		values[name] = value
	}
	baseDomain := values["E2E_BASE_DOMAIN"]
	runLabel := "gwc-" + values["RUN_ID"]
	suffix := "-" + runLabel + "." + baseDomain
	tunnelName := values["CLOUDFLARE_TUNNEL_NAME"] + "-" + runLabel

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	api, err := cloudflare.NewWithAPIToken(values["CLOUDFLARE_API_TOKEN"])
	if err != nil {
		return fmt.Errorf("create cloudflare client: %w", err)
	}

	zones, err := api.ListZones(ctx)
	if err != nil {
		return fmt.Errorf("list zones: %w", err)
	}
	var zone *cloudflare.Zone
	for i := range zones {
		if baseDomain == zones[i].Name || strings.HasSuffix(baseDomain, "."+zones[i].Name) {
			zone = &zones[i]
		}
	}
	if zone == nil {
		return fmt.Errorf("no zone found for E2E_BASE_DOMAIN")
	}

	preflight(ctx, api, zone.ID)

	records, _, err := api.ListDNSRecords(ctx, cloudflare.ZoneIdentifier(zone.ID), cloudflare.ListDNSRecordsParams{})
	if err != nil {
		return fmt.Errorf("list dns records: %w", err)
	}
	deleted := 0
	for _, record := range records {
		if !strings.HasSuffix(record.Name, suffix) {
			continue
		}
		if err := api.DeleteDNSRecord(ctx, cloudflare.ZoneIdentifier(zone.ID), record.ID); err != nil {
			return fmt.Errorf("delete %s record %s: %w", record.Type, redact(record.Name, baseDomain), err)
		}
		deleted++
		fmt.Printf("deleted %s record %s\n", record.Type, redact(record.Name, baseDomain))
	}

	tunnelRules, err := resetTunnel(ctx, api, values["CLOUDFLARE_ACCOUNT_ID"], tunnelName)
	if err != nil {
		return err
	}

	// verify, so callers can trust the result without another tool
	records, _, err = api.ListDNSRecords(ctx, cloudflare.ZoneIdentifier(zone.ID), cloudflare.ListDNSRecordsParams{})
	if err != nil {
		return fmt.Errorf("list dns records: %w", err)
	}
	remaining := 0
	for _, record := range records {
		if strings.HasSuffix(record.Name, suffix) {
			remaining++
		}
	}
	fmt.Printf("cleanup of %s done: deleted %d records, remaining %d records, tunnel rules %s\n", runLabel, deleted, remaining, tunnelRules)
	if remaining != 0 {
		return fmt.Errorf("%d records remain below %s", remaining, runLabel)
	}
	return nil
}

// resetTunnel leaves the run tunnel with the single catch all 404 rule. The
// tunnel itself is kept, the controller reuses it by name.
func resetTunnel(ctx context.Context, api *cloudflare.API, accountID string, tunnelName string) (string, error) {
	account := cloudflare.AccountIdentifier(accountID)
	tunnels, _, err := api.ListTunnels(ctx, account, cloudflare.TunnelListParams{
		Name:       tunnelName,
		IsDeleted:  ptr.To(false),
		ResultInfo: cloudflare.ResultInfo{Page: 1, PerPage: 1000},
	})
	if err != nil {
		return "", fmt.Errorf("list tunnels: %w", err)
	}
	for _, tunnel := range tunnels {
		if tunnel.Name != tunnelName {
			continue
		}
		_, err := api.UpdateTunnelConfiguration(ctx, account, cloudflare.TunnelConfigurationParams{
			TunnelID: tunnel.ID,
			Config: cloudflare.TunnelConfiguration{
				Ingress: []cloudflare.UnvalidatedIngressRule{{Service: "http_status:404"}},
			},
		})
		if err != nil {
			return "", fmt.Errorf("reset tunnel configuration: %w", err)
		}
		current, err := api.GetTunnelConfiguration(ctx, account, tunnel.ID)
		if err != nil {
			return "", fmt.Errorf("get tunnel configuration: %w", err)
		}
		return fmt.Sprintf("%d (only http_status:404)", len(current.Config.Ingress)), nil
	}
	return "not found (nothing to reset)", nil
}

// preflight warns about zone settings that make the suite fail through the
// edge. It is best effort, a token without the permission only gets a note.
func preflight(ctx context.Context, api *cloudflare.API, zoneID string) {
	zone := cloudflare.ZoneIdentifier(zoneID)
	for _, name := range []string{"always_use_https", "browser_check"} {
		setting, err := api.GetZoneSetting(ctx, zone, cloudflare.GetZoneSettingParams{Name: name})
		if err != nil {
			fmt.Printf("preflight: cannot read zone setting %s: %v\n", name, err)
			continue
		}
		if setting.Value == "on" {
			fmt.Printf("preflight WARNING: zone setting %s is on, it can break plain http conformance requests\n", name)
		} else {
			fmt.Printf("preflight: zone setting %s is %v\n", name, setting.Value)
		}
	}
	bot, err := api.GetBotManagement(ctx, zone)
	if err != nil {
		fmt.Printf("preflight: cannot read bot management: %v\n", err)
		return
	}
	if bot.FightMode != nil && *bot.FightMode {
		fmt.Println("preflight WARNING: Bot Fight Mode is on, it can challenge the conformance requests")
	} else {
		fmt.Println("preflight: Bot Fight Mode is off")
	}
}

// redact hides the base domain, it comes from the credentials file.
func redact(name string, baseDomain string) string {
	return strings.ReplaceAll(name, baseDomain, "<base-domain>")
}
