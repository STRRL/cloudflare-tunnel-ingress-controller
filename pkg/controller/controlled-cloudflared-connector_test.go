package controller

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestTunnelTokenNeedsRefresh(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

	secretRefreshedAt := func(at time.Time) *v1.Secret {
		return &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					tunnelTokenRefreshedAtAnnotation: at.Format(time.RFC3339),
				},
			},
		}
	}

	tests := []struct {
		name     string
		secret   *v1.Secret
		interval time.Duration
		expected bool
	}{
		{
			name:     "zero interval always refreshes",
			secret:   secretRefreshedAt(now),
			interval: 0,
			expected: true,
		},
		{
			name:     "negative interval always refreshes",
			secret:   secretRefreshedAt(now),
			interval: -time.Hour,
			expected: true,
		},
		{
			name:     "missing refresh stamp refreshes",
			secret:   &v1.Secret{},
			interval: time.Hour,
			expected: true,
		},
		{
			name: "unparseable refresh stamp refreshes",
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						tunnelTokenRefreshedAtAnnotation: "not-a-timestamp",
					},
				},
			},
			interval: time.Hour,
			expected: true,
		},
		{
			name:     "stamp inside the interval does not refresh",
			secret:   secretRefreshedAt(now.Add(-30 * time.Minute)),
			interval: time.Hour,
			expected: false,
		},
		{
			name:     "stamp exactly at the interval refreshes",
			secret:   secretRefreshedAt(now.Add(-time.Hour)),
			interval: time.Hour,
			expected: true,
		},
		{
			name:     "stamp older than the interval refreshes",
			secret:   secretRefreshedAt(now.Add(-2 * time.Hour)),
			interval: time.Hour,
			expected: true,
		},
		{
			name:     "stamp in the future does not refresh",
			secret:   secretRefreshedAt(now.Add(time.Hour)),
			interval: time.Hour,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tunnelTokenNeedsRefresh(tt.secret, tt.interval, now))
		})
	}
}

func TestTunnelTokenHash(t *testing.T) {
	t.Run("is stable for the same token", func(t *testing.T) {
		assert.Equal(t, tunnelTokenHash("a-token"), tunnelTokenHash("a-token"))
	})

	t.Run("differs for different tokens", func(t *testing.T) {
		assert.NotEqual(t, tunnelTokenHash("a-token"), tunnelTokenHash("another-token"))
	})

	t.Run("does not leak the token", func(t *testing.T) {
		assert.NotContains(t, tunnelTokenHash("a-token"), "a-token")
	})
}
