package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
)

func TestControlledCloudflaredDeploymentBuild(t *testing.T) {
	deployment := controlledCloudflaredDeployment{
		config: CloudflaredConfig{
			Image:           "cloudflare/cloudflared:latest",
			ImagePullPolicy: "IfNotPresent",
			Replicas:        2,
			Protocol:        "quic",
			ExtraArgs:       []string{"--post-quantum"},
		},
		tokenSecretVersion: "42",
		namespace:          "controller-system",
	}.build()

	assert.Equal(t, "controlled-cloudflared-connector", deployment.Name)
	assert.Equal(t, "controller-system", deployment.Namespace)
	require.NotNil(t, deployment.Spec.Replicas)
	assert.Equal(t, int32(2), *deployment.Spec.Replicas)
	assert.Equal(t, "42", deployment.Spec.Template.Annotations[tunnelTokenSecretVersionAnnotation])
	require.Len(t, deployment.Spec.Template.Spec.Containers, 1)

	container := deployment.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "cloudflare/cloudflared:latest", container.Image)
	assert.Equal(t, v1.PullIfNotPresent, container.ImagePullPolicy)
	assert.Equal(t, []string{
		"cloudflared",
		"--protocol",
		"quic",
		"--no-autoupdate",
		"tunnel",
		"--post-quantum",
		"--metrics",
		"0.0.0.0:44483",
		"run",
	}, container.Command)
	require.Len(t, container.Env, 1)
	require.NotNil(t, container.Env[0].ValueFrom)
	require.NotNil(t, container.Env[0].ValueFrom.SecretKeyRef)
	assert.Equal(t, tunnelTokenSecretName, container.Env[0].ValueFrom.SecretKeyRef.Name)
	assert.Equal(t, tunnelTokenSecretKey, container.Env[0].ValueFrom.SecretKeyRef.Key)
}

func TestControlledCloudflaredDeploymentBuildAntiAffinity(t *testing.T) {
	t.Run("disabled by default", func(t *testing.T) {
		deployment := controlledCloudflaredDeployment{
			config: CloudflaredConfig{Replicas: 3},
		}.build()

		assert.Nil(t, deployment.Spec.Template.Spec.Affinity)
	})

	t.Run("enabled sets required anti-affinity", func(t *testing.T) {
		deployment := controlledCloudflaredDeployment{
			config: CloudflaredConfig{Replicas: 3, PodAntiAffinity: true},
		}.build()

		require.NotNil(t, deployment.Spec.Template.Spec.Affinity)
		require.NotNil(t, deployment.Spec.Template.Spec.Affinity.PodAntiAffinity)

		terms := deployment.Spec.Template.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution
		require.Len(t, terms, 1)
		assert.Equal(t, "kubernetes.io/hostname", terms[0].TopologyKey)
		assert.Equal(t, map[string]string{"app": "controlled-cloudflared-connector"}, terms[0].LabelSelector.MatchLabels)
	})
}

func TestBuildPodAntiAffinity(t *testing.T) {
	t.Run("nil when disabled", func(t *testing.T) {
		assert.Nil(t, buildPodAntiAffinity("app", false))
	})

	t.Run("set when enabled", func(t *testing.T) {
		aff := buildPodAntiAffinity("my-app", true)
		require.NotNil(t, aff)
		terms := aff.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution
		require.Len(t, terms, 1)
		assert.Equal(t, "kubernetes.io/hostname", terms[0].TopologyKey)
		assert.Equal(t, map[string]string{"app": "my-app"}, terms[0].LabelSelector.MatchLabels)
	})
}

func TestControlledCloudflaredDeploymentBuildUsesConfiguredImage(t *testing.T) {
	deployment := controlledCloudflaredDeployment{
		config: CloudflaredConfig{
			Image:           "cloudflare/cloudflared:2026.7.0",
			ImagePullPolicy: "Always",
		},
	}.build()
	container := deployment.Spec.Template.Spec.Containers[0]

	assert.Equal(t, "cloudflare/cloudflared:2026.7.0", container.Image)
	assert.Equal(t, v1.PullAlways, container.ImagePullPolicy)
}
