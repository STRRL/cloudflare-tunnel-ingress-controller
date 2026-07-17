package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildCloudflaredCommand(t *testing.T) {
	tests := []struct {
		name      string
		protocol  string
		extraArgs []string
		expected  []string
	}{
		{
			name:      "basic command without extra args",
			protocol:  "auto",
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
			},
		},
		{
			name:      "command with post-quantum extra arg",
			protocol:  "quic",
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
			},
		},
		{
			name:      "command with multiple extra args",
			protocol:  "http2",
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
			},
		},
		{
			name:      "command with nil extra args",
			protocol:  "auto",
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
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildCloudflaredCommand(tt.protocol, tt.extraArgs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetDesiredReplicas(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv("CLOUDFLARED_REPLICA_COUNT", "")

		replicas, err := getDesiredReplicas()

		assert.NoError(t, err)
		assert.Equal(t, int32(1), replicas)
	})

	t.Run("invalid", func(t *testing.T) {
		t.Setenv("CLOUDFLARED_REPLICA_COUNT", "invalid")

		_, err := getDesiredReplicas()

		assert.Error(t, err)
	})
}
