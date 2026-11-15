package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
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

func TestTelemetryDirFromContainer(t *testing.T) {
	t.Run("no telemetry env means empty", func(t *testing.T) {
		container := &v1.Container{
			Env: []v1.EnvVar{
				{Name: "TUNNEL_TOKEN", Value: "token"},
			},
		}
		assert.Equal(t, "", telemetryDirFromContainer(container))
	})

	t.Run("telemetry env returns the directory", func(t *testing.T) {
		container := &v1.Container{
			Env: []v1.EnvVar{
				{Name: "TUNNEL_TOKEN", Value: "token"},
				{Name: "TUNNEL_DUCKMON_DIR", Value: "/var/lib/duckmon"},
			},
		}
		assert.Equal(t, "/var/lib/duckmon", telemetryDirFromContainer(container))
	})
}
