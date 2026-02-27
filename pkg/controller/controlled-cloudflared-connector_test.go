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

func TestCloudflaredConnectDeploymentTemplating(t *testing.T) {
	protocol := "auto"
	token := "test-token"
	namespace := "default"
	replicas := int32(1)
	extraArgs := []string{"--post-quantum"}

	deployment := cloudflaredConnectDeploymentTemplating(protocol, token, namespace, replicas, extraArgs)

	// Check if environment variable is set
	envVars := deployment.Spec.Template.Spec.Containers[0].Env
	tokenFound := false
	for _, env := range envVars {
		if env.Name == "TUNNEL_TOKEN" {
			assert.Equal(t, token, env.Value)
			tokenFound = true
			break
		}
	}
	assert.True(t, tokenFound, "TUNNEL_TOKEN environment variable not found")

	// Check if command does not contain token
	command := deployment.Spec.Template.Spec.Containers[0].Command
	for _, cmd := range command {
		assert.NotEqual(t, token, cmd, "Token should not be in command")
		assert.NotEqual(t, "--token", cmd, "--token flag should not be in command")
	}
}

func TestSlicesEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected bool
	}{
		{
			name:     "equal slices",
			a:        []string{"a", "b", "c"},
			b:        []string{"a", "b", "c"},
			expected: true,
		},
		{
			name:     "different length",
			a:        []string{"a", "b"},
			b:        []string{"a", "b", "c"},
			expected: false,
		},
		{
			name:     "different content",
			a:        []string{"a", "b", "c"},
			b:        []string{"a", "x", "c"},
			expected: false,
		},
		{
			name:     "empty slices",
			a:        []string{},
			b:        []string{},
			expected: true,
		},
		{
			name:     "nil slices",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			name:     "nil vs empty",
			a:        nil,
			b:        []string{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := slicesEqual(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}
