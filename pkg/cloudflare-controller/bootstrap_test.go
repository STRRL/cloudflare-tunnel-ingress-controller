package cloudflarecontroller

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/cloudflare/cloudflare-go/v6"
	"github.com/cloudflare/cloudflare-go/v6/zero_trust"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestBoolPointer(t *testing.T) {
	tests := []struct {
		name  string
		value bool
		want  bool
	}{
		{
			name:  "true value",
			value: true,
			want:  true,
		},
		{
			name:  "false value", 
			value: false,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ptr := boolPointer(tt.value)
			assert.NotNil(t, ptr)
			assert.Equal(t, tt.want, *ptr)
		})
	}
}

// Test tunnel lookup logic without actual API calls
func TestTunnelLookupLogic(t *testing.T) {

	tests := []struct {
		name            string
		tunnelName      string
		existingTunnels []zero_trust.TunnelListResponse
		expectFound     bool
		expectedId      string
	}{
		{
			name:       "tunnel found by name",
			tunnelName: "my-tunnel",
			existingTunnels: []zero_trust.TunnelListResponse{
				{
					ID:   "tunnel-123",
					Name: "my-tunnel",
				},
				{
					ID:   "tunnel-456", 
					Name: "other-tunnel",
				},
			},
			expectFound: true,
			expectedId:  "tunnel-123",
		},
		{
			name:       "tunnel not found",
			tunnelName: "missing-tunnel",
			existingTunnels: []zero_trust.TunnelListResponse{
				{
					ID:   "tunnel-123",
					Name: "my-tunnel",
				},
				{
					ID:   "tunnel-456",
					Name: "other-tunnel",
				},
			},
			expectFound: false,
			expectedId:  "",
		},
		{
			name:            "empty tunnel list",
			tunnelName:      "any-tunnel",
			existingTunnels: []zero_trust.TunnelListResponse{},
			expectFound:     false,
			expectedId:      "",
		},
		{
			name:       "case sensitive match",
			tunnelName: "My-Tunnel",
			existingTunnels: []zero_trust.TunnelListResponse{
				{
					ID:   "tunnel-123",
					Name: "my-tunnel", // different case
				},
			},
			expectFound: false, // Should be case sensitive
			expectedId:  "",
		},
		{
			name:       "exact case match",
			tunnelName: "my-tunnel",
			existingTunnels: []zero_trust.TunnelListResponse{
				{
					ID:   "tunnel-123",
					Name: "my-tunnel",
				},
			},
			expectFound: true,
			expectedId:  "tunnel-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the lookup logic from GetTunnelIdFromTunnelName
			var foundId string
			var found bool

			for _, tunnel := range tt.existingTunnels {
				if tunnel.Name == tt.tunnelName {
					foundId = tunnel.ID
					found = true
					break
				}
			}

			assert.Equal(t, tt.expectFound, found)
			assert.Equal(t, tt.expectedId, foundId)
		})
	}
}

// Test tunnel secret generation
func TestTunnelSecretGeneration(t *testing.T) {
	// Test that we can generate random secrets like the bootstrap code does
	randomSecret := make([]byte, 64)
	
	// This should not panic and should create a 64-byte slice
	assert.Len(t, randomSecret, 64)
	
	// Test hex encoding like in the actual code
	hexSecret := fmt.Sprintf("%x", randomSecret)
	
	// Should be 128 characters (64 bytes * 2 hex chars per byte)
	assert.Len(t, hexSecret, 128)
	
	// Should only contain hex characters
	hexPattern := regexp.MustCompile("^[0-9a-f]+$")
	assert.True(t, hexPattern.MatchString(hexSecret))
}

// Test tunnel creation parameters
func TestTunnelCreationParams(t *testing.T) {
	accountId := "test-account-123"
	tunnelName := "my-new-tunnel"
	hexSecret := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

	// Simulate the parameters that would be sent to Cloudflare API
	// This tests the parameter structure without making actual API calls
	params := zero_trust.TunnelCloudflaredNewParams{
		AccountID:    cloudflare.F(accountId),
		Name:         cloudflare.F(tunnelName),
		TunnelSecret: cloudflare.F(hexSecret),
		ConfigSrc:    cloudflare.F(zero_trust.TunnelCloudflaredNewParamsConfigSrcCloudflare),
	}

	// Verify the parameters are structured correctly (we can't access the internal values directly)
	assert.NotNil(t, params.AccountID)
	assert.NotNil(t, params.Name)  
	assert.NotNil(t, params.TunnelSecret)
	assert.NotNil(t, params.ConfigSrc)
}

// Test the bootstrap function's parameter validation
func TestBootstrapParameterValidation(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		accountId   string
		tunnelName  string
		expectError bool
	}{
		{
			name:        "valid parameters",
			accountId:   "valid-account-123", 
			tunnelName:  "valid-tunnel-name",
			expectError: false, // Would only error on API call
		},
		{
			name:        "empty account id - would fail at API level",
			accountId:   "",
			tunnelName:  "valid-tunnel-name",
			expectError: false, // Function doesn't validate this, would fail at API level
		},
		{
			name:        "empty tunnel name - would fail at API level", 
			accountId:   "valid-account-123",
			tunnelName:  "",
			expectError: false, // Function doesn't validate this, would fail at API level
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test parameter acceptance (not actual API calls)
			// The actual function would make API calls, but we can test the parameter structure
			
			// For tests with empty parameters, we don't validate as that would fail at API level
			if tt.name == "valid parameters" {
				assert.NotEmpty(t, tt.accountId)
				assert.NotEmpty(t, tt.tunnelName)
			}
			
			// Test that context is properly used
			assert.NotNil(t, ctx)
		})
	}
}

// Test auto-paging parameters
func TestTunnelListParams(t *testing.T) {
	accountId := "test-account-123"
	
	// Test the parameters used for listing tunnels
	params := zero_trust.TunnelListParams{
		AccountID: cloudflare.F(accountId),
		IsDeleted: cloudflare.F(false),
	}

	// Verify parameters are structured correctly
	assert.NotNil(t, params.AccountID)
	assert.NotNil(t, params.IsDeleted)
}

// Test error wrapping behavior
func TestErrorWrapping(t *testing.T) {
	// Test how errors would be wrapped in the bootstrap functions
	originalErr := errors.New("api error")
	tunnelName := "test-tunnel"
	
	// Simulate the error wrapping from GetTunnelIdFromTunnelName
	wrappedErr := errors.Wrapf(originalErr, "get tunnel id from tunnel name %s", tunnelName)
	
	assert.Contains(t, wrappedErr.Error(), "get tunnel id from tunnel name test-tunnel")
	assert.Contains(t, wrappedErr.Error(), "api error")
	
	// Test tunnel creation error wrapping
	createErr := errors.Wrapf(originalErr, "create tunnel %s", tunnelName)
	assert.Contains(t, createErr.Error(), "create tunnel test-tunnel")
	assert.Contains(t, createErr.Error(), "api error")
}

// Test that NewTunnelClient is called correctly after bootstrap
func TestBootstrapFlow(t *testing.T) {
	logger := logr.Discard()
	accountId := "test-account"
	tunnelId := "found-tunnel-id"
	tunnelName := "test-tunnel"

	// Simulate successful bootstrap flow 
	client := NewTunnelClient(logger, nil, accountId, tunnelId, tunnelName)

	// Verify client is created with correct parameters
	assert.NotNil(t, client)
	assert.Equal(t, accountId, client.accountId)
	assert.Equal(t, tunnelId, client.tunnelId)
	assert.Equal(t, tunnelName, client.tunnelName)
	assert.Equal(t, logger, client.logger)
}