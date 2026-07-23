package cloudflarecontroller

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/cloudflare/cloudflare-go"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

const ManagedRecordTXTPrefix = "_ctic_managed"

type ManagedRecordTXTContent struct {
	Controller string `json:"controller"`
	Tunnel     string `json:"tunnel"`
}

const ControllerIdentifier = "strrl.dev/cloudflare-tunnel-ingress-controller"

// managedTXTRecordComment is attached to ownership TXT records so they are
// recognizable in the Cloudflare dashboard. Informational only, not used for
// ownership checks.
const managedTXTRecordComment = "managed by " + ControllerIdentifier

// LegacyCommentFormat is the old comment-based ownership format.
// Used for migration: records with this comment are recognized as managed by this controller.
const LegacyCommentFormat = "managed by strrl.dev/cloudflare-tunnel-ingress-controller, tunnel [%s]"

type DNSOperationCreate struct {
	Hostname string
	Type     string
	Content  string
}

type DNSOperationUpdate struct {
	OldRecord cloudflare.DNSRecord
	Type      string
	Content   string
}

type DNSOperationDelete struct {
	OldRecord cloudflare.DNSRecord
}

// syncDNSRecord syncs the DNS records for the exposures.
// It creates, updates, and deletes the DNS records based on the exposures and the existing records.
//
// For each exposure hostname (e.g., 'dash.strrl.cloud'), it manages two records:
// - CNAME: dash.strrl.cloud -> <tunnel-id>.cfargotunnel.com (proxied)
// - TXT: _ctic_managed.dash.strrl.cloud -> JSON content identifying the controller and tunnel
//
// The TXT record is used to identify records managed by this controller.
// Deletion only occurs when a matching TXT record exists for the current tunnel.
func syncDNSRecord(
	logger logr.Logger,
	exposures []exposure.Exposure,
	existedCNAMERecords []cloudflare.DNSRecord,
	existedTXTRecords []cloudflare.DNSRecord,
	tunnelId string,
	tunnelName string,
) ([]DNSOperationCreate, []DNSOperationUpdate, []DNSOperationDelete, error) {
	effectiveExposures := exposure.Active(exposures)

	var toCreate []DNSOperationCreate
	var toUpdate []DNSOperationUpdate
	var toDelete []DNSOperationDelete

	expectedTXTContent, err := renderTXTContent(tunnelName)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "render managed record TXT content")
	}

	// Create or update CNAME/TXT records for active exposures
	for _, item := range effectiveExposures {
		txtRecordName := managedTXTRecordName(item.Hostname)

		// DNS management is delegated externally for this exposure: relinquish
		// ownership by cleaning up the records this controller created, so a
		// later ingress deletion can never claim an externally managed record.
		// The CNAME is only removed while it still points at this tunnel, if an
		// external system already repointed it the record must survive and only
		// the ownership TXT is dropped.
		if item.DisableDNSManagement {
			containsTXT, oldTXT := dnsRecordsContainsHostname(existedTXTRecords, txtRecordName)
			if containsTXT && oldTXT.Content == expectedTXTContent {
				containsCNAME, oldCNAME := dnsRecordsContainsHostname(existedCNAMERecords, item.Hostname)
				if containsCNAME && oldCNAME.Content == tunnelDomain(tunnelId) {
					toDelete = append(toDelete, DNSOperationDelete{
						OldRecord: oldCNAME,
					})
					logger.Info("DNS management disabled, deleting controller-owned CNAME record",
						"hostname", item.Hostname,
					)
				}
				toDelete = append(toDelete, DNSOperationDelete{
					OldRecord: oldTXT,
				})
				logger.Info("DNS management disabled, relinquishing ownership TXT record",
					"hostname", item.Hostname,
				)
			}
			continue
		}

		// Handle CNAME record
		containsCNAME, oldCNAME := dnsRecordsContainsHostname(existedCNAMERecords, item.Hostname)
		if containsCNAME {
			// Check if this record is managed by this controller
			hasTXTRecord, _ := dnsRecordsContainsHostname(existedTXTRecords, txtRecordName)
			if !hasTXTRecord {
				logger.Info("WARNING: overriding DNS record not managed by this controller",
					"hostname", item.Hostname,
					"existing-content", oldCNAME.Content,
				)
			}
			toUpdate = append(toUpdate, DNSOperationUpdate{
				OldRecord: oldCNAME,
				Type:      "CNAME",
				Content:   tunnelDomain(tunnelId),
			})
		} else {
			toCreate = append(toCreate, DNSOperationCreate{
				Hostname: item.Hostname,
				Type:     "CNAME",
				Content:  tunnelDomain(tunnelId),
			})
		}

		// Handle TXT record
		containsTXT, oldTXT := dnsRecordsContainsHostname(existedTXTRecords, txtRecordName)
		if containsTXT {
			toUpdate = append(toUpdate, DNSOperationUpdate{
				OldRecord: oldTXT,
				Type:      "TXT",
				Content:   expectedTXTContent,
			})
		} else {
			toCreate = append(toCreate, DNSOperationCreate{
				Hostname: txtRecordName,
				Type:     "TXT",
				Content:  expectedTXTContent,
			})
		}
	}

	// Delete CNAME/TXT records for removed exposures (only if managed by this tunnel)
	for _, cnameRecord := range existedCNAMERecords {
		containsInExposures, _ := exposureContainsHostname(effectiveExposures, cnameRecord.Name)
		if containsInExposures {
			continue
		}

		// Check if there's a corresponding TXT record managed by this tunnel
		txtRecordName := managedTXTRecordName(cnameRecord.Name)
		hasMatchingTXT, matchingTXTRecord := findMatchingTXTRecord(existedTXTRecords, txtRecordName, expectedTXTContent)

		// Only delete if we have a matching TXT record (proves ownership)
		if hasMatchingTXT {
			toDelete = append(toDelete, DNSOperationDelete{
				OldRecord: cnameRecord,
			})
			toDelete = append(toDelete, DNSOperationDelete{
				OldRecord: matchingTXTRecord,
			})
		}
	}

	return toCreate, toUpdate, toDelete, nil
}

// migrateLegacyDNSRecords handles migration from the old comment-based ownership to TXT-based ownership.
// It identifies CNAME records that use the legacy comment format and are no longer in active exposures,
// and returns delete operations for them. Records already tracked by TXT records are skipped
// (they are handled by syncDNSRecord).
func migrateLegacyDNSRecords(
	logger logr.Logger,
	exposures []exposure.Exposure,
	existedCNAMERecords []cloudflare.DNSRecord,
	existedTXTRecords []cloudflare.DNSRecord,
	tunnelName string,
) ([]DNSOperationDelete, error) {
	effectiveExposures := exposure.Active(exposures)

	legacyComment := renderLegacyComment(tunnelName)
	expectedTXTContent, err := renderTXTContent(tunnelName)
	if err != nil {
		return nil, errors.Wrap(err, "render managed record TXT content")
	}

	var toDelete []DNSOperationDelete
	for _, cnameRecord := range existedCNAMERecords {
		// Skip records still in active exposures
		containsInExposures, _ := exposureContainsHostname(effectiveExposures, cnameRecord.Name)
		if containsInExposures {
			continue
		}

		// Skip records already tracked by TXT (handled by syncDNSRecord)
		txtRecordName := managedTXTRecordName(cnameRecord.Name)
		hasTXTRecord, _ := findMatchingTXTRecord(existedTXTRecords, txtRecordName, expectedTXTContent)
		if hasTXTRecord {
			continue
		}

		// Delete if the CNAME has the legacy comment format matching the current tunnel
		if cnameRecord.Comment == legacyComment {
			logger.Info("migrating legacy comment-based record for deletion",
				"hostname", cnameRecord.Name,
			)
			toDelete = append(toDelete, DNSOperationDelete{
				OldRecord: cnameRecord,
			})
		}
	}

	return toDelete, nil
}

func dnsRecordsContainsHostname(records []cloudflare.DNSRecord, hostname string) (bool, cloudflare.DNSRecord) {
	for _, item := range records {
		if item.Name == hostname {
			return true, item
		}
	}
	return false, cloudflare.DNSRecord{}
}

// managedTXTRecordName returns the name of the ownership TXT record that tracks
// the given hostname, e.g. "_ctic_managed.dash.strrl.cloud".
func managedTXTRecordName(hostname string) string {
	return fmt.Sprintf("%s.%s", ManagedRecordTXTPrefix, hostname)
}

// findMatchingTXTRecord returns the TXT record matching both name and content,
// used to prove this controller/tunnel owns the corresponding CNAME record.
func findMatchingTXTRecord(records []cloudflare.DNSRecord, name string, content string) (bool, cloudflare.DNSRecord) {
	for _, record := range records {
		if record.Name == name && record.Content == content {
			return true, record
		}
	}
	return false, cloudflare.DNSRecord{}
}

func exposureContainsHostname(exposures []exposure.Exposure, hostname string) (bool, exposure.Exposure) {
	for _, item := range exposures {
		if item.Hostname == hostname {
			return true, item
		}
	}
	return false, exposure.Exposure{}
}

const WellKnownTunnelDomainFormat = "%s.cfargotunnel.com"

func tunnelDomain(tunnelId string) string {
	return strings.ToLower(fmt.Sprintf(WellKnownTunnelDomainFormat, tunnelId))
}

func renderLegacyComment(tunnelName string) string {
	return fmt.Sprintf(LegacyCommentFormat, tunnelName)
}

func renderTXTContent(tunnelName string) (string, error) {
	content := ManagedRecordTXTContent{
		Controller: ControllerIdentifier,
		Tunnel:     tunnelName,
	}
	jsonBytes, err := json.Marshal(content)
	if err != nil {
		return "", errors.Wrap(err, "marshal managed record TXT content")
	}
	return string(jsonBytes), nil
}

func parseTXTContent(content string) (*ManagedRecordTXTContent, error) {
	var result ManagedRecordTXTContent
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, err
	}
	return &result, nil
}
