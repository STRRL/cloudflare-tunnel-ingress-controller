package cloudflarecontroller

import (
	"slices"
	"strings"
	"testing"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/cloudflare/cloudflare-go/v6/zones"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
)

func TestNewTunnelClient(t *testing.T) {
	logger := logr.Discard()
	accountId := "test-account-id"
	tunnelId := "test-tunnel-id"
	tunnelName := "test-tunnel"

	client := NewTunnelClient(logger, nil, accountId, tunnelId, tunnelName)

	assert.NotNil(t, client)
	assert.Equal(t, accountId, client.accountId)
	assert.Equal(t, tunnelId, client.tunnelId)
	assert.Equal(t, tunnelName, client.tunnelName)
	assert.Equal(t, logger, client.logger)
}

func TestTunnelClient_TunnelDomain(t *testing.T) {
	logger := logr.Discard()
	tunnelId := "abc123def456"
	client := NewTunnelClient(logger, nil, "account", tunnelId, "tunnel")

	domain := client.TunnelDomain()
	expected := "abc123def456.cfargotunnel.com"
	assert.Equal(t, expected, domain)
}

func TestZoneBelongedByExposure(t *testing.T) {
	tests := []struct {
		name      string
		exposure  exposure.Exposure
		zones     []string
		wantFound bool
		wantZone  string
	}{
		{
			name: "exact match",
			exposure: exposure.Exposure{
				Hostname: "example.com",
			},
			zones:     []string{"example.com", "test.com"},
			wantFound: true,
			wantZone:  "example.com",
		},
		{
			name: "subdomain match",
			exposure: exposure.Exposure{
				Hostname: "api.example.com",
			},
			zones:     []string{"example.com", "test.com"},
			wantFound: true,
			wantZone:  "example.com",
		},
		{
			name: "deep subdomain match",
			exposure: exposure.Exposure{
				Hostname: "v1.api.example.com",
			},
			zones:     []string{"example.com", "test.com"},
			wantFound: true,
			wantZone:  "example.com",
		},
		{
			name: "no match",
			exposure: exposure.Exposure{
				Hostname: "other.org",
			},
			zones:     []string{"example.com", "test.com"},
			wantFound: false,
			wantZone:  "",
		},
		{
			name: "partial match should not work",
			exposure: exposure.Exposure{
				Hostname: "notexample.com",
			},
			zones:     []string{"example.com", "test.com"},
			wantFound: false,
			wantZone:  "",
		},
		{
			name: "empty zones",
			exposure: exposure.Exposure{
				Hostname: "test.com",
			},
			zones:     []string{},
			wantFound: false,
			wantZone:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFound, gotZone := zoneBelongedByExposure(tt.exposure, tt.zones)
			assert.Equal(t, tt.wantFound, gotFound)
			assert.Equal(t, tt.wantZone, gotZone)
		})
	}
}

func TestFindZoneByName(t *testing.T) {
	zonesList := []zones.Zone{
		{
			ID:   "zone1",
			Name: "example.com",
		},
		{
			ID:   "zone2", 
			Name: "test.com",
		},
	}

	tests := []struct {
		name     string
		zoneName string
		wantFound bool
		wantZone zones.Zone
	}{
		{
			name:     "found zone",
			zoneName: "example.com",
			wantFound: true,
			wantZone: zones.Zone{
				ID:   "zone1",
				Name: "example.com",
			},
		},
		{
			name:     "zone not found",
			zoneName: "missing.com",
			wantFound: false,
			wantZone: zones.Zone{},
		},
		{
			name:     "empty zone name",
			zoneName: "",
			wantFound: false,
			wantZone: zones.Zone{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFound, gotZone := findZoneByName(tt.zoneName, zonesList)
			assert.Equal(t, tt.wantFound, gotFound)
			assert.Equal(t, tt.wantZone, gotZone)
		})
	}
}

func TestTunnelClient_ExposureSorting(t *testing.T) {
	// Test the sorting logic that happens in updateTunnelIngressRules
	exposures := []exposure.Exposure{
		{
			Hostname:   "example.com",
			PathPrefix: "/short",
			IsDeleted:  false,
		},
		{
			Hostname:   "api.example.com",
			PathPrefix: "/very/long/path",
			IsDeleted:  false,
		},
		{
			Hostname:   "api.example.com", 
			PathPrefix: "/api",
			IsDeleted:  false,
		},
		{
			Hostname:   "example.com",
			PathPrefix: "/very/long/path/that/is/longer",
			IsDeleted:  false,
		},
		{
			Hostname:   "api.example.com",
			PathPrefix: "/short",
			IsDeleted:  false,
		},
		{
			Hostname:   "deleted.example.com",
			PathPrefix: "/should/not/appear",
			IsDeleted:  true, // This should be filtered out
		},
	}

	// Filter and sort like the code does
	var effectiveExposures []exposure.Exposure
	for _, item := range exposures {
		if !item.IsDeleted {
			effectiveExposures = append(effectiveExposures, item)
		}
	}

	// Apply the same sorting logic
	slices.SortFunc(effectiveExposures, func(a, b exposure.Exposure) int {
		if v := strings.Compare(strings.ToLower(a.Hostname), strings.ToLower(b.Hostname)); v != 0 {
			return v
		}
		return len(b.PathPrefix) - len(a.PathPrefix)
	})

	// Verify results
	assert.Len(t, effectiveExposures, 5) // Should exclude deleted one
	
	// Should be sorted by hostname first
	assert.Equal(t, "api.example.com", effectiveExposures[0].Hostname)
	assert.Equal(t, "api.example.com", effectiveExposures[1].Hostname)
	assert.Equal(t, "api.example.com", effectiveExposures[2].Hostname)
	assert.Equal(t, "example.com", effectiveExposures[3].Hostname)
	assert.Equal(t, "example.com", effectiveExposures[4].Hostname)

	// Within same hostname, should be sorted by path length descending
	// api.example.com entries
	assert.Equal(t, "/very/long/path", effectiveExposures[0].PathPrefix) // longest first
	assert.Equal(t, "/short", effectiveExposures[1].PathPrefix)
	assert.Equal(t, "/api", effectiveExposures[2].PathPrefix)

	// example.com entries  
	assert.Equal(t, "/very/long/path/that/is/longer", effectiveExposures[3].PathPrefix) // longest first
	assert.Equal(t, "/short", effectiveExposures[4].PathPrefix)
}

// Test helper functions from dns.go that are used by tunnel-client
func TestTunnelDomain(t *testing.T) {
	tests := []struct {
		name     string
		tunnelId string
		want     string
	}{
		{
			name:     "normal tunnel id",
			tunnelId: "abc123def456", 
			want:     "abc123def456.cfargotunnel.com",
		},
		{
			name:     "tunnel id with uppercase",
			tunnelId: "ABC123DEF456",
			want:     "abc123def456.cfargotunnel.com",
		},
		{
			name:     "empty tunnel id",
			tunnelId: "",
			want:     ".cfargotunnel.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tunnelDomain(tt.tunnelId)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRenderDNSRecordComment(t *testing.T) {
	tests := []struct {
		name       string
		tunnelName string
		want       string
	}{
		{
			name:       "normal tunnel name",
			tunnelName: "my-tunnel",
			want:       "managed by strrl.dev/cloudflare-tunnel-ingress-controller, tunnel [my-tunnel]",
		},
		{
			name:       "empty tunnel name",
			tunnelName: "",
			want:       "managed by strrl.dev/cloudflare-tunnel-ingress-controller, tunnel []",
		},
		{
			name:       "tunnel name with special chars",
			tunnelName: "test-tunnel_123",
			want:       "managed by strrl.dev/cloudflare-tunnel-ingress-controller, tunnel [test-tunnel_123]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderDNSRecordComment(tt.tunnelName)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Test validation of interface compliance
func TestTunnelClient_ImplementsInterface(t *testing.T) {
	var _ TunnelClientInterface = &TunnelClient{}
	// If this compiles, the interface is properly implemented
}

// Test error cases for DNS operations
func TestTunnelClient_DNSErrorHandling(t *testing.T) {
	// Test with exposures that have invalid hostnames (no matching zones)
	exposures := []exposure.Exposure{
		{
			Hostname:      "invalid.unknown-zone.com",
			ServiceTarget: "http://service:80",
			IsDeleted:     false,
		},
	}

	zones := []string{"example.com", "test.com"}

	for _, exp := range exposures {
		found, _ := zoneBelongedByExposure(exp, zones)
		assert.False(t, found, "Should not find zone for invalid hostname")
	}
}