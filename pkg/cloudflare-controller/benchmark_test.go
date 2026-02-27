package cloudflarecontroller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/cloudflare/cloudflare-go"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
)

// BenchmarkPutExposures establishes a baseline for sequential vs concurrent processing
func BenchmarkPutExposures(b *testing.B) {
	// Simulate network latency typical of API calls
	const latency = 10 * time.Millisecond

	// Create a mock Cloudflare API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate network delay
		time.Sleep(latency)

		w.Header().Set("Content-Type", "application/json")

		// Helper to write JSON success response
		respondSuccess := func(result interface{}) {
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"result":  result,
				"errors":  []interface{}{},
			}); err != nil {
				panic(err)
			}
		}

		// Handle various API endpoints
		path := r.URL.Path
		method := r.Method

		// Match List Zones
		// e.g. GET /client/v4/zones
		if method == "GET" && strings.HasSuffix(path, "/zones") {
			respondSuccess([]map[string]interface{}{
				{"id": "zone-123", "name": "example.com"},
			})
			return
		}

		// Match List DNS Records
		// e.g. GET /client/v4/zones/zone-123/dns_records
		if method == "GET" && strings.Contains(path, "/dns_records") {
			// Return empty list so we create new records for every exposure
			respondSuccess([]interface{}{})
			return
		}

		// Match Create DNS Record
		// e.g. POST /client/v4/zones/zone-123/dns_records
		if method == "POST" && strings.Contains(path, "/dns_records") {
			respondSuccess(map[string]interface{}{
				"id": "new-record-id",
			})
			return
		}

		// Match Update Tunnel Config
		// e.g. PUT /client/v4/accounts/account-id/tunnels/tunnel-id/configurations
		if method == "PUT" && strings.Contains(path, "/configurations") {
			respondSuccess(map[string]interface{}{})
			return
		}

		// Fallback for unexpected calls
		respondSuccess(map[string]interface{}{})
	}))
	defer server.Close()

	// Configure HTTP client with higher MaxIdleConnsPerHost to support concurrency
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 20,
		},
	}

	// Initialize Cloudflare client pointing to our mock server
	cfClient, err := cloudflare.New(
		"api-key",
		"api-email",
		cloudflare.BaseURL(server.URL),
		cloudflare.HTTPClient(httpClient),
		cloudflare.UsingRateLimit(100), // Increase rate limit for benchmark
	)
	require.NoError(b, err)

	// Create TunnelClient
	client := NewTunnelClient(logr.Discard(), cfClient, "account-id", "tunnel-id", "tunnel-name")

	// Create a set of exposures that will trigger multiple API calls
	numExposures := 10
	exposures := make([]exposure.Exposure, numExposures)
	for i := 0; i < numExposures; i++ {
		exposures[i] = exposure.Exposure{
			Hostname:      fmt.Sprintf("app-%d.example.com", i),
			ServiceTarget: "http://service:80",
			PathPrefix:    "/",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := client.PutExposures(context.Background(), exposures)
		require.NoError(b, err)
	}
}
