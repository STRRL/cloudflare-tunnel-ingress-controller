package controller

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	t.Run("single replica has no anti-affinity", func(t *testing.T) {
		dep := cloudflaredConnectDeploymentTemplating("quic", "tok", "ns", 1, nil)

		assert.Equal(t, "controlled-cloudflared-connector", dep.Name)
		assert.Equal(t, "ns", dep.Namespace)
		assert.Equal(t, int32(1), *dep.Spec.Replicas)
		assert.Nil(t, dep.Spec.Template.Spec.Affinity, "single replica should have no affinity")
	})

	t.Run("multiple replicas have anti-affinity", func(t *testing.T) {
		dep := cloudflaredConnectDeploymentTemplating("quic", "tok", "ns", 3, nil)

		assert.Equal(t, int32(3), *dep.Spec.Replicas)
		require.NotNil(t, dep.Spec.Template.Spec.Affinity)
		require.NotNil(t, dep.Spec.Template.Spec.Affinity.PodAntiAffinity)

		terms := dep.Spec.Template.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution
		require.Len(t, terms, 1)
		assert.Equal(t, "kubernetes.io/hostname", terms[0].TopologyKey)
		assert.Equal(t, map[string]string{"app": "controlled-cloudflared-connector"}, terms[0].LabelSelector.MatchLabels)
	})

	t.Run("labels are consistent across object meta, selector, and template", func(t *testing.T) {
		dep := cloudflaredConnectDeploymentTemplating("quic", "tok", "ns", 2, nil)

		expectedLabels := map[string]string{
			"app": "controlled-cloudflared-connector",
			"strrl.dev/cloudflare-tunnel-ingress-controller": "controlled-cloudflared-connector",
		}
		assert.Equal(t, expectedLabels, dep.Labels)
		assert.Equal(t, expectedLabels, dep.Spec.Selector.MatchLabels)
		assert.Equal(t, expectedLabels, dep.Spec.Template.Labels)
	})

	t.Run("container uses provided protocol and token", func(t *testing.T) {
		dep := cloudflaredConnectDeploymentTemplating("http2", "my-token", "default", 1, []string{"--post-quantum"})

		require.Len(t, dep.Spec.Template.Spec.Containers, 1)
		c := dep.Spec.Template.Spec.Containers[0]
		assert.Equal(t, "controlled-cloudflared-connector", c.Name)
		assert.Contains(t, c.Command, "http2")
		assert.Contains(t, c.Command, "my-token")
		assert.Contains(t, c.Command, "--post-quantum")
	})
}

func TestBuildPodAntiAffinity(t *testing.T) {
	t.Run("nil for single replica", func(t *testing.T) {
		assert.Nil(t, buildPodAntiAffinity("app", 1))
	})

	t.Run("nil for zero replicas", func(t *testing.T) {
		assert.Nil(t, buildPodAntiAffinity("app", 0))
	})

	t.Run("set for multiple replicas", func(t *testing.T) {
		aff := buildPodAntiAffinity("my-app", 3)
		require.NotNil(t, aff)
		terms := aff.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution
		require.Len(t, terms, 1)
		assert.Equal(t, "kubernetes.io/hostname", terms[0].TopologyKey)
		assert.Equal(t, map[string]string{"app": "my-app"}, terms[0].LabelSelector.MatchLabels)
	})
}

func TestAffinityEqual(t *testing.T) {
	aff1 := &v1.Affinity{
		PodAntiAffinity: &v1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "test"},
					},
					TopologyKey: "kubernetes.io/hostname",
				},
			},
		},
	}
	aff2 := &v1.Affinity{
		PodAntiAffinity: &v1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "test"},
					},
					TopologyKey: "kubernetes.io/hostname",
				},
			},
		},
	}

	assert.True(t, affinityEqual(nil, nil))
	assert.False(t, affinityEqual(aff1, nil))
	assert.False(t, affinityEqual(nil, aff1))
	assert.True(t, affinityEqual(aff1, aff2))
}
