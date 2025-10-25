package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCloudflaredCommand(t *testing.T) {
	tests := []struct {
		name      string
		protocol  string
		token     string
		extraArgs []string
		expected  []string
	}{
		{
			name:      "basic command without extra args",
			protocol:  "auto",
			token:     "test-token",
			extraArgs: []string{},
			expected: []string{
				"cloudflared",
				"--protocol",
				"auto",
				"--no-autoupdate",
				"tunnel",
				"--metrics",
				"0.0.0.0:44483",
				"run",
				"--token",
				"test-token",
			},
		},
		{
			name:      "command with post-quantum extra arg",
			protocol:  "quic",
			token:     "test-token",
			extraArgs: []string{"--post-quantum"},
			expected: []string{
				"cloudflared",
				"--protocol",
				"quic",
				"--no-autoupdate",
				"tunnel",
				"--post-quantum",
				"--metrics",
				"0.0.0.0:44483",
				"run",
				"--token",
				"test-token",
			},
		},
		{
			name:      "command with multiple extra args",
			protocol:  "http2",
			token:     "test-token",
			extraArgs: []string{"--post-quantum", "--edge-ip-version", "4"},
			expected: []string{
				"cloudflared",
				"--protocol",
				"http2",
				"--no-autoupdate",
				"tunnel",
				"--post-quantum",
				"--edge-ip-version",
				"4",
				"--metrics",
				"0.0.0.0:44483",
				"run",
				"--token",
				"test-token",
			},
		},
		{
			name:      "command with nil extra args",
			protocol:  "auto",
			token:     "test-token",
			extraArgs: nil,
			expected: []string{
				"cloudflared",
				"--protocol",
				"auto",
				"--no-autoupdate",
				"tunnel",
				"--metrics",
				"0.0.0.0:44483",
				"run",
				"--token",
				"test-token",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getCloudflaredCommand(tt.protocol, tt.token, tt.extraArgs)
			assert.Equal(t, tt.expected, result)
		})
	}
}
