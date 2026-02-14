package cloudflarecontroller

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"text/template"

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
	logger              logr.Logger
	cfClient            *cloudflare.API
	accountId           string
	tunnelId            string
	tunnelName          string
	dnsCommentTemplate  *template.Template // nil if disabled (empty template string)
	dnsCommentTemplateS string             // raw template string for logging
}

// DNSCommentTemplateData contains the variables available in the DNS comment template.
// See https://developers.cloudflare.com/dns/manage-dns-records/reference/record-attributes/
// for comment length limits per Cloudflare plan (Free: 100, Pro/Business/Enterprise: 500 chars).
type DNSCommentTemplateData struct {
	TunnelName string // Name of the Cloudflare Tunnel
	TunnelId   string // ID of the Cloudflare Tunnel
	Hostname   string // DNS record hostname (e.g. "app.example.com")
}

func NewTunnelClient(logger logr.Logger, cfClient *cloudflare.API, accountId string, tunnelId string, tunnelName string, dnsCommentTemplate string) *TunnelClient {
	tc := &TunnelClient{
		logger:              logger,
		cfClient:            cfClient,
		accountId:           accountId,
		tunnelId:            tunnelId,
		tunnelName:          tunnelName,
		dnsCommentTemplateS: dnsCommentTemplate,
	}
	if dnsCommentTemplate != "" {
		tmpl, err := template.New("dns-comment").Parse(dnsCommentTemplate)
		if err != nil {
			logger.Error(err, "failed to parse dns-comment-template, DNS comments will be disabled", "template", dnsCommentTemplate)
		} else {
			tc.dnsCommentTemplate = tmpl
		}
	}
	return tc
}

// renderDNSComment renders the DNS comment for a given hostname using the configured template.
// Returns empty string if the template is disabled or rendering fails.
func (t *TunnelClient) renderDNSComment(hostname string) string {
	if t.dnsCommentTemplate == nil {
		return ""
	}
	data := DNSCommentTemplateData{
		TunnelName: t.tunnelName,
		TunnelId:   t.tunnelId,
		Hostname:   hostname,
	}
	var buf bytes.Buffer
	if err := t.dnsCommentTemplate.Execute(&buf, data); err != nil {
		t.logger.Error(err, "failed to render dns comment template", "hostname", hostname)
		return ""
	}
	comment := buf.String()

	// Warn about comment length.
	// Cloudflare enforces per-plan limits: Free=100, Pro/Business/Enterprise=500 chars.
	// See https://developers.cloudflare.com/dns/manage-dns-records/reference/record-attributes/
	if len(comment) > 100 {
		t.logger.Info("WARNING: rendered DNS comment exceeds 100 characters (Cloudflare Free plan limit). "+
			"Pro/Business/Enterprise plans allow up to 500 characters. "+
			"If your plan does not support this length, the API call may fail.",
			"hostname", hostname, "commentLength", len(comment),
		)
	}
	return comment
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
		return errors.Wrapf(err, "list CNAME records for zone %s", zone.Name)
	}

	allTxtDnsRecords, _, err := t.cfClient.ListDNSRecords(ctx, cloudflare.ResourceIdentifier(zone.ID), cloudflare.ListDNSRecordsParams{
		Type: "TXT",
	})
	if err != nil {
		return errors.Wrapf(err, "list TXT records for zone %s", zone.Name)
	}

	// Filter to only include TXT records managed by this controller
	var txtDnsRecords []cloudflare.DNSRecord
	for _, record := range allTxtDnsRecords {
		if strings.HasPrefix(record.Name, ManagedRecordTXTPrefix+".") {
			txtDnsRecords = append(txtDnsRecords, record)
		}
	}

	toCreate, toUpdate, toDelete, err := syncDNSRecord(t.logger, exposures, cnameDnsRecords, txtDnsRecords, t.tunnelId, t.tunnelName)
	if err != nil {
		return errors.Wrap(err, "sync DNS records")
	}
	t.logger.V(3).Info("sync DNS records", "to-create", toCreate, "to-update", toUpdate, "to-delete", toDelete)

	for _, item := range toCreate {
		t.logger.Info("create DNS record", "type", item.Type, "hostname", item.Hostname, "content", item.Content)
		params := cloudflare.CreateDNSRecordParams{
			Type:    item.Type,
			Name:    item.Hostname,
			Content: item.Content,
			Proxied: cloudflare.BoolPtr(item.Type == "CNAME"),
			TTL:     1,
		}
		// Add comment to CNAME records if template is configured.
		// Comments are informational only; ownership is tracked via TXT records.
		// See https://developers.cloudflare.com/dns/manage-dns-records/reference/record-attributes/
		if item.Type == "CNAME" {
			if comment := t.renderDNSComment(item.Hostname); comment != "" {
				params.Comment = comment
			}
		}
		_, err := t.cfClient.CreateDNSRecord(ctx, cloudflare.ResourceIdentifier(zone.ID), params)
		if err != nil {
			return errors.Wrapf(err, "create DNS record for zone %s, hostname %s", zone.Name, item.Hostname)
		}
	}

	for _, item := range toUpdate {
		t.logger.Info("update DNS record", "id", item.OldRecord.ID, "type", item.Type, "hostname", item.OldRecord.Name, "content", item.Content)
		params := cloudflare.UpdateDNSRecordParams{
			ID:      item.OldRecord.ID,
			Type:    item.Type,
			Name:    item.OldRecord.Name,
			Content: item.Content,
			Proxied: cloudflare.BoolPtr(item.Type == "CNAME"),
			TTL:     1,
		}
		// Add comment to CNAME records if template is configured.
		if item.Type == "CNAME" {
			if comment := t.renderDNSComment(item.OldRecord.Name); comment != "" {
				params.Comment = &comment
			}
		}
		_, err := t.cfClient.UpdateDNSRecord(ctx, cloudflare.ResourceIdentifier(zone.ID), params)
		if err != nil {
			return errors.Wrapf(err, "update DNS record for zone %s, hostname %s", zone.Name, item.OldRecord.Name)
		}
	}

	// Migrate legacy comment-based records (separate from normal sync)
	legacyDeletes := migrateLegacyDNSRecords(t.logger, exposures, cnameDnsRecords, txtDnsRecords, t.tunnelName)
	toDelete = append(toDelete, legacyDeletes...)

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
