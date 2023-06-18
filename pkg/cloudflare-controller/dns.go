package cloudflarecontroller

import (
	"fmt"
	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/cloudflare/cloudflare-go"
	"strings"
)

const ManagedCNAMERecordCommentMark = "managed by cloudflare-tunnel-ingress-controller"

type DNSOperationCreate struct {
	Hostname string
	Type     string
	Content  string
	Comment  string
}

type DNSOperationUpdate struct {
	OldRecord cloudflare.DNSRecord
	Type      string
	Content   string
	Comment   string
}

type DNSOperationDelete struct {
	OldRecord cloudflare.DNSRecord
}

func syncDNSRecord(exposures []exposure.Exposure, existedCNAMERecords []cloudflare.DNSRecord, tunnelId string) ([]DNSOperationCreate, []DNSOperationUpdate, []DNSOperationDelete, error) {
	var effectiveExposures []exposure.Exposure
	for _, item := range exposures {
		if !item.IsDeleted {
			effectiveExposures = append(effectiveExposures, item)
		}
	}

	var toCreate []DNSOperationCreate
	var toUpdate []DNSOperationUpdate

	for _, item := range effectiveExposures {
		contains, old := dnsRecordsContainsHostname(existedCNAMERecords, item.Hostname)

		if contains {
			toUpdate = append(toUpdate, DNSOperationUpdate{
				OldRecord: old,
				Type:      "CNAME",
				Content:   tunnelDomain(tunnelId),
				Comment:   ManagedCNAMERecordCommentMark,
			})
		} else {
			toCreate = append(toCreate, DNSOperationCreate{
				Hostname: item.Hostname,
				Type:     "CNAME",
				Content:  tunnelDomain(tunnelId),
				Comment:  ManagedCNAMERecordCommentMark,
			})
		}
	}

	var toDelete []DNSOperationDelete
	for _, item := range existedCNAMERecords {
		contains, _ := exposureContainsHostname(effectiveExposures, item.Name)
		if !contains {
			if item.Comment == ManagedCNAMERecordCommentMark {
				toDelete = append(toDelete, DNSOperationDelete{
					OldRecord: item,
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
