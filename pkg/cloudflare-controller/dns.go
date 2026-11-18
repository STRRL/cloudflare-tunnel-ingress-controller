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

// wildcardTXTLabel replaces "*" in ownership TXT names to suppress Cloudflare's
// warning that a star is only a wildcard as the leftmost "*." prefix.
//
// The leading underscore keeps this distinct from a literal Ingress host:
// Kubernetes hostnames are DNS-1123 (letters, digits, hyphen), so
// "*.example.com" maps to _ctic_managed._wildcard.example.com while
// "wildcard.example.com" maps to _ctic_managed.wildcard.example.com.
const wildcardTXTLabel = "_wildcard"

type ManagedRecordTXTContent struct {
	Controller string `json:"controller"`
	Tunnel     string `json:"tunnel"`
}

const ControllerIdentifier = "strrl.dev/cloudflare-tunnel-ingress-controller"

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
// - TXT: _ctic_managed.dash.strrl.cloud -> RFC 1035-quoted JSON identifying the controller and tunnel
//
// Wildcard hostnames replace "*" with "_wildcard" in the TXT name, e.g.
// *.example.com -> _ctic_managed._wildcard.example.com, so Cloudflare does
// not warn about a non-prefix asterisk. That label also cannot collide with
// a literal wildcard.example.com host. The TXT record proves ownership;
// deletion only occurs when a matching TXT record exists for the current tunnel.
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

	// Every path of an Ingress becomes its own Exposure, so a hostname is
	// usually visited several times while it resolves to the same pair of DNS
	// records. Handling it once keeps the operation list and the migration logs
	// free of repetition. DisableDNSManagement belongs in the key because it
	// selects which branch below runs.
	handledHostnames := map[string]struct{}{}

	// Create or update CNAME/TXT records for active exposures
	for _, item := range effectiveExposures {
		hostnameKey := fmt.Sprintf("%t/%s", item.DisableDNSManagement, item.Hostname)
		if _, handled := handledHostnames[hostnameKey]; handled {
			continue
		}
		handledHostnames[hostnameKey] = struct{}{}

		txtRecordName := managedTXTRecordName(item.Hostname)

		// DNS management is delegated externally for this exposure: relinquish
		// ownership by cleaning up the records this controller created, so a
		// later ingress deletion can never claim an externally managed record.
		// The CNAME is only removed while it still points at this tunnel, if an
		// external system already repointed it the record must survive and only
		// the ownership TXT is dropped.
		if item.DisableDNSManagement {
			ownedTXTs := ownedTXTRecords(existedTXTRecords, item.Hostname, expectedTXTContent)
			if len(ownedTXTs) > 0 {
				containsCNAME, oldCNAME := dnsRecordsContainsHostname(existedCNAMERecords, item.Hostname)
				if containsCNAME && oldCNAME.Content == tunnelDomain(tunnelId) {
					toDelete = append(toDelete, DNSOperationDelete{
						OldRecord: oldCNAME,
					})
					logger.Info("DNS management disabled, deleting controller-owned CNAME record",
						"hostname", item.Hostname,
					)
				}
				for _, oldTXT := range ownedTXTs {
					toDelete = append(toDelete, DNSOperationDelete{
						OldRecord: oldTXT,
					})
				}
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
			if !hasOwnershipTXTName(existedTXTRecords, item.Hostname) {
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

		// Handle TXT record at the current name (* replaced with _wildcard).
		containsTXT, oldTXT := dnsRecordsContainsHostname(existedTXTRecords, txtRecordName)
		if containsTXT {
			if oldTXT.Content != expectedTXTContent && txtContentEqual(oldTXT.Content, expectedTXTContent) {
				logger.Info("migrating ownership TXT record to RFC 1035 quoted form",
					"hostname", txtRecordName,
				)
			}
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

		// Drop leftover TXT names that still embed "*" and trigger Cloudflare's warning.
		for _, leftover := range leftoverLegacyWildcardTXTRecords(existedTXTRecords, item.Hostname, expectedTXTContent) {
			logger.Info("migrating wildcard ownership TXT record",
				"hostname", item.Hostname,
				"from", leftover.Name,
				"to", txtRecordName,
			)
			toDelete = append(toDelete, DNSOperationDelete{
				OldRecord: leftover,
			})
		}
	}

	// Delete CNAME/TXT records for removed exposures (only if managed by this tunnel)
	for _, cnameRecord := range existedCNAMERecords {
		containsInExposures, _ := exposureContainsHostname(effectiveExposures, cnameRecord.Name)
		if containsInExposures {
			continue
		}

		ownedTXTs := ownedTXTRecords(existedTXTRecords, cnameRecord.Name, expectedTXTContent)
		if len(ownedTXTs) == 0 {
			continue
		}
		toDelete = append(toDelete, DNSOperationDelete{
			OldRecord: cnameRecord,
		})
		for _, matchingTXTRecord := range ownedTXTs {
			toDelete = append(toDelete, DNSOperationDelete{
				OldRecord: matchingTXTRecord,
			})
		}
	}

	return deduplicateDNSCreates(toCreate), deduplicateDNSUpdates(toUpdate), deduplicateDNSDeletes(toDelete), nil
}

func deduplicateDNSCreates(operations []DNSOperationCreate) []DNSOperationCreate {
	var result []DNSOperationCreate
	seen := map[string]struct{}{}
	for _, operation := range operations {
		key := operation.Type + "\x00" + operation.Hostname
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, operation)
	}
	return result
}

func deduplicateDNSUpdates(operations []DNSOperationUpdate) []DNSOperationUpdate {
	var result []DNSOperationUpdate
	seen := map[string]struct{}{}
	for _, operation := range operations {
		key := dnsRecordKey(operation.OldRecord)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, operation)
	}
	return result
}

func deduplicateDNSDeletes(operations []DNSOperationDelete) []DNSOperationDelete {
	var result []DNSOperationDelete
	seen := map[string]struct{}{}
	for _, operation := range operations {
		key := dnsRecordKey(operation.OldRecord)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, operation)
	}
	return result
}

func dnsRecordKey(record cloudflare.DNSRecord) string {
	if record.ID != "" {
		return record.ID
	}
	return record.Type + "\x00" + record.Name + "\x00" + record.Content
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
		if len(ownedTXTRecords(existedTXTRecords, cnameRecord.Name, expectedTXTContent)) > 0 {
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
// the given hostname. For example:
//
//	dash.strrl.cloud     -> _ctic_managed.dash.strrl.cloud
//	wildcard.example.com -> _ctic_managed.wildcard.example.com
//	*.example.com        -> _ctic_managed._wildcard.example.com
//
// The "*" label is replaced with "_wildcard" to suppress Cloudflare's warning
// that a star is only a wildcard as the leftmost "*." prefix. The underscore
// keeps this distinct from a literal "wildcard." hostname: Ingress hosts are
// DNS-1123 and cannot contain underscores.
func managedTXTRecordName(hostname string) string {
	return fmt.Sprintf("%s.%s", ManagedRecordTXTPrefix, dnsNameForTXT(hostname))
}

func dnsNameForTXT(hostname string) string {
	if strings.HasPrefix(hostname, "*.") {
		return wildcardTXTLabel + "." + hostname[2:]
	}
	return hostname
}

// ownershipTXTNames returns the current TXT name and, for wildcards, the
// historical name that embedded a literal "*" in the record.
func ownershipTXTNames(hostname string) []string {
	current := managedTXTRecordName(hostname)
	legacy := fmt.Sprintf("%s.%s", ManagedRecordTXTPrefix, hostname)
	if current == legacy {
		return []string{current}
	}
	return []string{current, legacy}
}

func hasOwnershipTXTName(records []cloudflare.DNSRecord, hostname string) bool {
	for _, name := range ownershipTXTNames(hostname) {
		if contains, _ := dnsRecordsContainsHostname(records, name); contains {
			return true
		}
	}
	return false
}

func ownedTXTRecords(records []cloudflare.DNSRecord, hostname string, expectedContent string) []cloudflare.DNSRecord {
	var owned []cloudflare.DNSRecord
	for _, name := range ownershipTXTNames(hostname) {
		if has, rec := findMatchingTXTRecord(records, name, expectedContent); has {
			owned = append(owned, rec)
		}
	}
	return owned
}

// leftoverLegacyWildcardTXTRecords returns owned TXT records that still use the
// historical "*" name, so that wildcard ownership migrates automatically. They
// are reported as deletions, and updateDNSCNAMERecordForZone applies creations
// before deletions, so the current `_wildcard` record is always in place before
// the old one goes away.
func leftoverLegacyWildcardTXTRecords(records []cloudflare.DNSRecord, hostname string, expectedContent string) []cloudflare.DNSRecord {
	current := managedTXTRecordName(hostname)
	var leftovers []cloudflare.DNSRecord
	for _, name := range ownershipTXTNames(hostname) {
		if name == current {
			continue
		}
		if has, rec := findMatchingTXTRecord(records, name, expectedContent); has {
			leftovers = append(leftovers, rec)
		}
	}
	return leftovers
}

// findMatchingTXTRecord returns the TXT record matching both name and content,
// used to prove this controller/tunnel owns the corresponding CNAME record.
// Quoted and unquoted payloads are treated as equal so records created before
// RFC 1035 quoting was added still prove ownership.
func findMatchingTXTRecord(records []cloudflare.DNSRecord, name string, content string) (bool, cloudflare.DNSRecord) {
	for _, record := range records {
		if record.Name == name && txtContentEqual(record.Content, content) {
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
	return quoteTXTContent(string(jsonBytes)), nil
}

func parseTXTContent(content string) (*ManagedRecordTXTContent, error) {
	var result ManagedRecordTXTContent
	if err := json.Unmarshal([]byte(unquoteTXTContent(content)), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func txtContentEqual(a, b string) bool {
	return unquoteTXTContent(a) == unquoteTXTContent(b)
}

// txtCharacterStringLimit is the maximum length of one RFC 1035 character-string.
// Cloudflare splits longer payloads on its own, so splitting them here keeps the
// stored content identical to what was sent and avoids a permanent mismatch
// between the expected and the observed record.
const txtCharacterStringLimit = 255

// quoteTXTContent renders s as RFC 1035 character-strings, escaping inner quotes
// and backslashes so the TXT RDATA is unambiguous. Payloads that exceed a single
// character-string are split into space separated chunks, which receivers
// concatenate back into the original value.
func quoteTXTContent(s string) string {
	if s == "" {
		return `""`
	}
	var b strings.Builder
	b.Grow(len(s) + 2)
	for start := 0; start < len(s); start += txtCharacterStringLimit {
		if start > 0 {
			b.WriteByte(' ')
		}
		writeTXTCharacterString(&b, s[start:min(start+txtCharacterStringLimit, len(s))])
	}
	return b.String()
}

func writeTXTCharacterString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\', '"':
			b.WriteByte('\\')
		}
		b.WriteByte(s[i])
	}
	b.WriteByte('"')
}

// unquoteTXTContent returns the logical TXT payload, accepting both the
// historical unquoted JSON and the RFC 1035 quoted form, including the multiple
// character-strings Cloudflare produces for long values. Input that is not
// well-formed quoting is returned as-is rather than half decoded.
func unquoteTXTContent(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '"' {
		return s
	}
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == ' ' {
			i++
			continue
		}
		if s[i] != '"' {
			return s
		}
		i++
		closed := false
		for i < len(s) {
			if s[i] == '\\' && i+1 < len(s) {
				b.WriteByte(s[i+1])
				i += 2
				continue
			}
			if s[i] == '"' {
				i++
				closed = true
				break
			}
			b.WriteByte(s[i])
			i++
		}
		if !closed {
			return s
		}
	}
	return b.String()
}
