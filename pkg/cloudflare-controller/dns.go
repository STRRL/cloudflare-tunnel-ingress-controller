package cloudflarecontroller

import (
	"fmt"
	"strings"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/cloudflare/cloudflare-go"
)

const ManagedRecordTXTContentFormat = "managed by strrl.dev/cloudflare-tunnel-ingress-controller, tunnel [%s]"

// ctic, abbr for cloudflare tunnel ingress controller, does not include the dot
const ManagedRecordTXTPrefix = "_ctic_managed"

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
// for example, if we want to expose a service with hostname 'dash.strrl.cloud',
// it will create a CNAME record and a TXT record
//
// - CNAME: dash.strrl.cloud -> <tunnel-id>.cfargotunnel.com
//
// - TXT: _ctic_managed.dash.strrl.cloud -> managed by strrl.dev/cloudflare-tunnel-ingress-controller, tunnel [<tunnel-name>]
//
// the CNAME record is **required** for the cloudflare tunnel to work,
// the TXT record is used to identify the domain is managed by this controller.
//
// this controller is designed as "authoritative" for the DNS records,
// it will ALWAYS create/update CNAME records and TXT records best effort,
// so it will override the existing records **whatever they are managed by this controller or not**.
//
// but things are different for the deletion,
// this controller will only delete the CNAME record,
// and only when the TXT record is deleted, it will delete the CNAME record.
func syncDNSRecord(
	exposures []exposure.Exposure,
	existedCNAMERecords []cloudflare.DNSRecord,
	existedTXTRecords []cloudflare.DNSRecord,
	tunnelId string,
	tunnelName string) ([]DNSOperationCreate, []DNSOperationUpdate, []DNSOperationDelete, error) {
	// effective exposures would be set online later
	var effectiveExposures []exposure.Exposure
	for _, item := range exposures {
		if !item.IsDeleted {
			effectiveExposures = append(effectiveExposures, item)
		}
	}

	var toCreate []DNSOperationCreate
	var toUpdate []DNSOperationUpdate
	var toDelete []DNSOperationDelete

	// create or update CNAME/TXT record for exposures should online
	for _, item := range effectiveExposures {
		containsCNAME, oldCNAME := dnsRecordsContainsHostname(existedCNAMERecords, item.Hostname)

		if containsCNAME {
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

		containsTXT, oldTXT := dnsRecordsContainsHostname(existedTXTRecords, fmt.Sprintf("%s.%s", ManagedRecordTXTPrefix, item.Hostname))
		if containsTXT {
			toUpdate = append(toUpdate, DNSOperationUpdate{
				OldRecord: oldTXT,
				Type:      "TXT",
				Content:   fmt.Sprintf(ManagedRecordTXTContentFormat, tunnelName),
			})
		} else {
			toCreate = append(toCreate, DNSOperationCreate{
				Hostname: fmt.Sprintf("%s.%s", ManagedRecordTXTPrefix, item.Hostname),
				Type:     "TXT",
				Content:  fmt.Sprintf(ManagedRecordTXTContentFormat, tunnelName),
			})
		}
	}

	// delete CNAME/TXT record for exposures should offline
	for _, cnameRecord := range existedCNAMERecords {
		containsCNAME, _ := exposureContainsHostname(effectiveExposures, cnameRecord.Name)
		if !containsCNAME {
			// Check if there's a corresponding TXT record
			var targetTXTRecord *cloudflare.DNSRecord
			for _, txtRecord := range existedTXTRecords {
				txtRecord := txtRecord
				if txtRecord.Name == fmt.Sprintf("%s.%s", ManagedRecordTXTPrefix, cnameRecord.Name) {
					targetTXTRecord = &txtRecord
					break
				}
			}
			if targetTXTRecord != nil {
				toDelete = append(toDelete, DNSOperationDelete{
					OldRecord: cnameRecord,
				})
				toDelete = append(toDelete, DNSOperationDelete{
					OldRecord: *targetTXTRecord,
				})
			}
		}
	}

	return toCreate, toUpdate, toDelete, nil
}

func dnsRecordsContainsHostname(records []cloudflare.DNSRecord, hostname string) (bool, cloudflare.DNSRecord) {
	for _, item := range records {
		if item.Name == hostname {
			return true, item
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
