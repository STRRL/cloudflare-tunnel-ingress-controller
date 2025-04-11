package cloudflarecontroller

import (
	"context"
	"slices"
	"strings"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/cloudflare/cloudflare-go"
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
	cfClient   *cloudflare.API
	accountId  string
	tunnelId   string
	tunnelName string
}

func NewTunnelClient(logger logr.Logger, cfClient *cloudflare.API, accountId string, tunnelId string, tunnelName string) *TunnelClient {
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
	var ingressRules []cloudflare.UnvalidatedIngressRule

	var effectiveExposures []exposure.Exposure
	for _, item := range exposures {
		if !item.IsDeleted {
			effectiveExposures = append(effectiveExposures, item)
		}
	}

	for _, item := range effectiveExposures {
		ingress, err := fromExposureToCloudflareIngress(ctx, item)
		if err != nil {
			return errors.Wrapf(err, "transform to cloudflare ingress")
		}
		ingressRules = append(ingressRules, *ingress)
	}

	// sort the rules by hostnames first for prettiness, then by path length in descending order
	// to ensure "precedence will be given first to the longest matching path".
	slices.SortFunc(ingressRules, func(a, b cloudflare.UnvalidatedIngressRule) int {
		if v := strings.Compare(strings.ToLower(a.Hostname), strings.ToLower(b.Hostname)); v != 0 {
			return v
		}
		return len(b.Path) - len(a.Path)
	})

	// at last, append a default 404 service as default route
	ingressRules = append(ingressRules, cloudflare.UnvalidatedIngressRule{
		Service: "http_status:404",
	})

	t.logger.V(3).Info("update cloudflare tunnel config", "ingress-rules", ingressRules)

	_, err := t.cfClient.UpdateTunnelConfiguration(ctx,
		cloudflare.ResourceIdentifier(t.accountId),
		cloudflare.TunnelConfigurationParams{
			TunnelID: t.tunnelId,
			Config: cloudflare.TunnelConfiguration{
				Ingress: ingressRules,
			},
		},
	)

	if err != nil {
		return errors.Wrap(err, "update cloudflare tunnel config")
	}
	return nil
}

func (t *TunnelClient) updateDNSCNAMERecord(ctx context.Context, exposures []exposure.Exposure) error {
	t.logger.V(3).Info("list zones")
	zones, err := t.cfClient.ListZones(ctx)
	if err != nil {
		return errors.Wrap(err, "list cloudflare zones")
	}

	var zoneNames []string
	for _, zone := range zones {
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
		ok, zone := findZoneByName(zoneName, zones)
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

func (t *TunnelClient) updateDNSCNAMERecordForZone(ctx context.Context, exposures []exposure.Exposure, zone cloudflare.Zone) error {
	cnameDnsRecords, _, err := t.cfClient.ListDNSRecords(ctx, cloudflare.ResourceIdentifier(zone.ID), cloudflare.ListDNSRecordsParams{
		Type: "CNAME",
	})
	if err != nil {
		return errors.Wrapf(err, "list DNS records for zone %s", zone.Name)
	}

	txtDnsRecords, _, err := t.cfClient.ListDNSRecords(ctx, cloudflare.ResourceIdentifier(zone.ID), cloudflare.ListDNSRecordsParams{
		Type: "TXT",
	})
	if err != nil {
		return errors.Wrapf(err, "list TXT records for zone %s", zone.Name)
	}

	toCreate, toUpdate, toDelete, err := syncDNSRecord(exposures, cnameDnsRecords, txtDnsRecords, t.tunnelId, t.tunnelName)
	if err != nil {
		return errors.Wrap(err, "sync DNS records")
	}
	t.logger.V(3).Info("sync DNS records", "to-create", toCreate, "to-update", toUpdate, "to-delete", toDelete)

	for _, item := range toCreate {
		t.logger.Info("create DNS record", "type", item.Type, "hostname", item.Hostname, "content", item.Content)
		_, err := t.cfClient.CreateDNSRecord(ctx, cloudflare.ResourceIdentifier(zone.ID), cloudflare.CreateDNSRecordParams{
			Type:    item.Type,
			Name:    item.Hostname,
			Content: item.Content,
			Proxied: cloudflare.BoolPtr(item.Type == "CNAME"),
			TTL:     1,
		})
		if err != nil {
			return errors.Wrapf(err, "create DNS record for zone %s, hostname %s", zone.Name, item.Hostname)
		}
	}

	for _, item := range toUpdate {
		t.logger.Info("update DNS record", "id", item.OldRecord.ID, "type", item.Type, "hostname", item.OldRecord.Name, "content", item.Content)

		_, err := t.cfClient.UpdateDNSRecord(ctx, cloudflare.ResourceIdentifier(zone.ID), cloudflare.UpdateDNSRecordParams{
			ID:      item.OldRecord.ID,
			Type:    item.Type,
			Name:    item.OldRecord.Name,
			Content: item.Content,
			Proxied: cloudflare.BoolPtr(item.Type == "CNAME"),
			TTL:     1,
		})
		if err != nil {
			return errors.Wrapf(err, "update DNS record for zone %s, hostname %s", zone.Name, item.OldRecord.Name)
		}
	}

	for _, item := range toDelete {
		t.logger.Info("delete DNS record", "id", item.OldRecord.ID, "type", item.OldRecord.Type, "hostname", item.OldRecord.Name, "content", item.OldRecord.Content)
		err := t.cfClient.DeleteDNSRecord(ctx, cloudflare.ResourceIdentifier(zone.ID), item.OldRecord.ID)
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

func findZoneByName(zoneName string, zones []cloudflare.Zone) (bool, cloudflare.Zone) {
	for _, zone := range zones {
		if zone.Name == zoneName {
			return true, zone
		}
	}
	return false, cloudflare.Zone{}
}

func (t *TunnelClient) FetchTunnelToken(ctx context.Context) (string, error) {
	return t.cfClient.GetTunnelToken(ctx, cloudflare.ResourceIdentifier(t.accountId), t.tunnelId)
}
