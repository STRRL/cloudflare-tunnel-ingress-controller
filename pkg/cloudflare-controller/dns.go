package cloudflarecontroller

import (
	"fmt"
	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/cloudflare/cloudflare-go"
	"strings"
)

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

func syncDNSRecord(exposures []exposure.Exposure, existedRecords []cloudflare.DNSRecord, tunnelId string) ([]DNSOperationCreate, []DNSOperationUpdate, []DNSOperationDelete, error) {
	var toCreate []DNSOperationCreate
	var toUpdate []DNSOperationUpdate

	for _, item := range exposures {
		contains, old := dnsRecordsContainsHostname(existedRecords, item.Hostname)

		if contains {
			toUpdate = append(toUpdate, DNSOperationUpdate{
				OldRecord: old,
				Type:      "CNAME",
				Content:   tunnelDomain(tunnelId),
				Comment:   "managed by cloudflare-tunnel-ingress-controller",
			})
		} else {
			toCreate = append(toCreate, DNSOperationCreate{
				Hostname: item.Hostname,
				Type:     "CNAME",
				Content:  tunnelDomain(tunnelId),
				Comment:  "managed by cloudflare-tunnel-ingress-controller",
			})
		}
	}

	var toDelete []DNSOperationDelete
	for _, item := range existedRecords {
		contains, _ := exposureContainsHostname(exposures, item.Name)
		if !contains {
			toDelete = append(toDelete, DNSOperationDelete{
				OldRecord: item,
			})
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
