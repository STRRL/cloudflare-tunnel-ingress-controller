package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
)

func TestBuildCloudflaredCommand(t *testing.T) {
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
			result := buildCloudflaredCommand(tt.protocol, tt.token, tt.extraArgs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCloudflaredConnectDeploymentTemplating(t *testing.T) {
	tests := []struct {
		name            string
		protocol        string
		token           string
		namespace       string
		replicas        int32
		extraArgs       []string
		envVars         map[string]string
		expectedImage   string
		expectedPullPol v1.PullPolicy
	}{
		{
			name:            "default values",
			protocol:        "auto",
			token:           "test-token",
			namespace:       "test-ns",
			replicas:        2,
			extraArgs:       []string{"--post-quantum"},
			envVars:         map[string]string{},
			expectedImage:   "cloudflare/cloudflared:latest",
			expectedPullPol: v1.PullIfNotPresent,
		},
		{
			name:      "custom environment variables",
			protocol:  "quic",
			token:     "another-token",
			namespace: "custom-ns",
			replicas:  3,
			extraArgs: []string{},
			envVars: map[string]string{
				"CLOUDFLARED_IMAGE":              "my-custom-image:v1",
				"CLOUDFLARED_IMAGE_PULL_POLICY": "Always",
			},
			expectedImage:   "my-custom-image:v1",
			expectedPullPol: v1.PullAlways,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}
			// If not provided in tt.envVars, ensure they are cleared to test defaults
			if _, ok := tt.envVars["CLOUDFLARED_IMAGE"]; !ok {
				t.Setenv("CLOUDFLARED_IMAGE", "")
			}
			if _, ok := tt.envVars["CLOUDFLARED_IMAGE_PULL_POLICY"]; !ok {
				t.Setenv("CLOUDFLARED_IMAGE_PULL_POLICY", "")
			}

			deployment := cloudflaredConnectDeploymentTemplating(tt.protocol, tt.token, tt.namespace, tt.replicas, tt.extraArgs)

			assert.NotNil(t, deployment)
			assert.Equal(t, "controlled-cloudflared-connector", deployment.Name)
			assert.Equal(t, tt.namespace, deployment.Namespace)
			assert.Equal(t, tt.replicas, *deployment.Spec.Replicas)

			// Labels
			expectedLabels := map[string]string{
				"app": "controlled-cloudflared-connector",
				"strrl.dev/cloudflare-tunnel-ingress-controller": "controlled-cloudflared-connector",
			}
			assert.Equal(t, expectedLabels, deployment.Labels)
			assert.Equal(t, expectedLabels, deployment.Spec.Selector.MatchLabels)
			assert.Equal(t, expectedLabels, deployment.Spec.Template.Labels)

			// Container
			assert.Len(t, deployment.Spec.Template.Spec.Containers, 1)
			container := deployment.Spec.Template.Spec.Containers[0]
			assert.Equal(t, "controlled-cloudflared-connector", container.Name)
			assert.Equal(t, tt.expectedImage, container.Image)
			assert.Equal(t, tt.expectedPullPol, container.ImagePullPolicy)

			// Command
			expectedCommand := buildCloudflaredCommand(tt.protocol, tt.token, tt.extraArgs)
			assert.Equal(t, expectedCommand, container.Command)
		})
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
