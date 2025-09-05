package cloudflarecontroller

import (
	"context"
	"slices"
	"strings"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/cloudflare/cloudflare-go/v6"
	"github.com/cloudflare/cloudflare-go/v6/dns"
	"github.com/cloudflare/cloudflare-go/v6/zero_trust"
	"github.com/cloudflare/cloudflare-go/v6/zones"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

type TunnelClientInterface interface {
	PutExposures(ctx context.Context, exposures []exposure.Exposure) error
	TunnelDomain() string
	FetchTunnelToken(ctx context.Context) (string, error)
}

var _ TunnelClientInterface = &TunnelClient{}

type TunnelClient struct {
	logger     logr.Logger
	cfClient   *cloudflare.Client
	accountId  string
	tunnelId   string
	tunnelName string
}

func NewTunnelClient(logger logr.Logger, cfClient *cloudflare.Client, accountId string, tunnelId string, tunnelName string) *TunnelClient {
	return &TunnelClient{logger: logger, cfClient: cfClient, accountId: accountId, tunnelId: tunnelId, tunnelName: tunnelName}
}

func (t *TunnelClient) PutExposures(ctx context.Context, exposures []exposure.Exposure) error {
	err := t.updateTunnelIngressRules(ctx, exposures)
	if err != nil {
		return errors.Wrap(err, "update tunnel ingress rules")
	}

	err = t.updateDNSCNAMERecord(ctx, exposures)
	if err != nil {
		return errors.Wrap(err, "update DNS CNAME record")
	}
	return nil
}

func (t *TunnelClient) TunnelDomain() string {
	return tunnelDomain(t.tunnelId)
}

func (t *TunnelClient) updateTunnelIngressRules(ctx context.Context, exposures []exposure.Exposure) error {
	var ingressRules []zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress

	var effectiveExposures []exposure.Exposure
	for _, item := range exposures {
		if !item.IsDeleted {
			effectiveExposures = append(effectiveExposures, item)
		}
	}

	// Sort the exposures by hostname first, then by path length in descending order
	// to ensure "precedence will be given first to the longest matching path".
	slices.SortFunc(effectiveExposures, func(a, b exposure.Exposure) int {
		if v := strings.Compare(strings.ToLower(a.Hostname), strings.ToLower(b.Hostname)); v != 0 {
			return v
		}
		return len(b.PathPrefix) - len(a.PathPrefix)
	})

	for _, item := range effectiveExposures {
		ingress, err := fromExposureToCloudflareIngress(ctx, item)
		if err != nil {
			return errors.Wrapf(err, "transform to cloudflare ingress")
		}
		ingressRules = append(ingressRules, ingress)
	}

	// at last, append a default 404 service as default route
	ingressRules = append(ingressRules, zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress{
		Service: cloudflare.F("http_status:404"),
	})

	t.logger.V(3).Info("update cloudflare tunnel config", "ingress-rules", ingressRules)

	_, err := t.cfClient.ZeroTrust.Tunnels.Cloudflared.Configurations.Update(ctx, t.tunnelId, zero_trust.TunnelCloudflaredConfigurationUpdateParams{
		AccountID: cloudflare.F(t.accountId),
		Config: cloudflare.F(zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfig{
			Ingress: cloudflare.F(ingressRules),
		}),
	})

	if err != nil {
		return errors.Wrap(err, "update cloudflare tunnel config")
	}
	return nil
}

func (t *TunnelClient) updateDNSCNAMERecord(ctx context.Context, exposures []exposure.Exposure) error {
	t.logger.V(3).Info("list zones")
	var zonesList []zones.Zone
	pager := t.cfClient.Zones.ListAutoPaging(ctx, zones.ZoneListParams{})

	for pager.Next() {
		zone := pager.Current()
		zonesList = append(zonesList, zone)
	}

	if pager.Err() != nil {
		return errors.Wrap(pager.Err(), "list cloudflare zones")
	}

	var zoneNames []string
	for _, zone := range zonesList {
		zoneNames = append(zoneNames, zone.Name)
	}
	t.logger.V(3).Info("zones", "zones", zoneNames)

	var exposuresByZone = make(map[string][]exposure.Exposure)
	for _, item := range exposures {
		ok, zone := zoneBelongedByExposure(item, zoneNames)
		if ok {
			exposuresByZone[zone] = append(exposuresByZone[zone], item)
		} else {
			return errors.Errorf("hostname %s not belong to any zone", item.Hostname)
		}
	}

	for zoneName, items := range exposuresByZone {
		ok, zone := findZoneByName(zoneName, zonesList)
		if !ok {
			return errors.Errorf("zone %s not found", zoneName)
		}
		err := t.updateDNSCNAMERecordForZone(ctx, items, zone)
		if err != nil {
			return errors.Wrapf(err, "update DNS CNAME record for zone %s", zoneNames)
		}
	}
	return nil
}

func (t *TunnelClient) updateDNSCNAMERecordForZone(ctx context.Context, exposures []exposure.Exposure, zone zones.Zone) error {
	var cnameDnsRecords []dns.RecordResponse
	pager := t.cfClient.DNS.Records.ListAutoPaging(ctx, dns.RecordListParams{
		ZoneID: cloudflare.F(zone.ID),
		Type:   cloudflare.F(dns.RecordListParamsTypeCNAME),
	})

	for pager.Next() {
		record := pager.Current()
		cnameDnsRecords = append(cnameDnsRecords, record)
	}

	err := pager.Err()
	if err != nil {
		return errors.Wrapf(err, "list DNS records for zone %s", zone.Name)
	}
	toCreate, toUpdate, toDelete, err := syncDNSRecord(exposures, cnameDnsRecords, t.tunnelId, t.tunnelName)
	if err != nil {
		return errors.Wrap(err, "sync DNS records")
	}
	t.logger.V(3).Info("sync DNS records", "to-create", toCreate, "to-update", toUpdate, "to-delete", toDelete)

	for _, item := range toCreate {
		t.logger.Info("create DNS record", "type", item.Type, "hostname", item.Hostname, "content", item.Content)
		_, err := t.cfClient.DNS.Records.New(ctx, dns.RecordNewParams{
			ZoneID: cloudflare.F(zone.ID),
			Body: dns.RecordNewParamsBody{
				Type:    cloudflare.F(dns.RecordNewParamsBodyTypeCNAME),
				Name:    cloudflare.F(item.Hostname),
				Content: cloudflare.F(item.Content),
				Comment: cloudflare.F(item.Comment),
				Proxied: cloudflare.F(true),
				TTL:     cloudflare.F(dns.TTL(1)),
			},
		})
		if err != nil {
			return errors.Wrapf(err, "create DNS record for zone %s, hostname %s", zone.Name, item.Hostname)
		}
	}

	for _, item := range toUpdate {

		if item.OldRecord.Comment != renderDNSRecordComment(t.tunnelName) {
			t.logger.Info("WARNING, the origin DNS record is not managed by this controller, it would be changed to managed record",
				"origin-record", item.OldRecord,
			)
		}

		t.logger.Info("update DNS record", "id", item.OldRecord.ID, "type", item.Type, "hostname", item.OldRecord.Name, "content", item.Content)

		_, err := t.cfClient.DNS.Records.Update(ctx, item.OldRecord.ID, dns.RecordUpdateParams{
			ZoneID: cloudflare.F(zone.ID),
			Body: dns.RecordUpdateParamsBody{
				Type:    cloudflare.F(dns.RecordUpdateParamsBodyTypeCNAME),
				Name:    cloudflare.F(item.OldRecord.Name),
				Content: cloudflare.F(item.Content),
				Comment: cloudflare.F(item.Comment),
				Proxied: cloudflare.F(true),
				TTL:     cloudflare.F(dns.TTL(1)),
			},
		})
		if err != nil {
			return errors.Wrapf(err, "update DNS record for zone %s, hostname %s", zone.Name, item.OldRecord.Name)
		}
	}

	for _, item := range toDelete {
		t.logger.Info("delete DNS record", "id", item.OldRecord.ID, "type", item.OldRecord.Type, "hostname", item.OldRecord.Name, "content", item.OldRecord.Content)
		_, err := t.cfClient.DNS.Records.Delete(ctx, item.OldRecord.ID, dns.RecordDeleteParams{
			ZoneID: cloudflare.F(zone.ID),
		})
		if err != nil {
			return errors.Wrapf(err, "delete DNS record for zone %s, hostname %s", zone.Name, item.OldRecord.Name)
		}
	}

	return nil
}

func zoneBelongedByExposure(exposure exposure.Exposure, zones []string) (bool, string) {
	hostnameDomain := Domain{Name: exposure.Hostname}

	for _, zone := range zones {
		zoneDomain := Domain{Name: zone}
		if hostnameDomain.IsSubDomainOf(zoneDomain) || hostnameDomain.Name == zoneDomain.Name {
			return true, zone
		}
	}
	return false, ""
}

func findZoneByName(zoneName string, zonesList []zones.Zone) (bool, zones.Zone) {
	for _, zone := range zonesList {
		if zone.Name == zoneName {
			return true, zone
		}
	}
	return false, zones.Zone{}
}

func (t *TunnelClient) FetchTunnelToken(ctx context.Context) (string, error) {
	token, err := t.cfClient.ZeroTrust.Tunnels.Cloudflared.Token.Get(ctx, t.tunnelId, zero_trust.TunnelCloudflaredTokenGetParams{
		AccountID: cloudflare.F(t.accountId),
	})
	if err != nil {
		return "", err
	}
	return *token, nil
}
