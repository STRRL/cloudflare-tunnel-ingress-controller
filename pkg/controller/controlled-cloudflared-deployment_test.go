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
	t.Run("single replica has no anti-affinity", func(t *testing.T) {
		deployment := controlledCloudflaredDeployment{
			config: CloudflaredConfig{Replicas: 1},
		}.build()

		assert.Nil(t, deployment.Spec.Template.Spec.Affinity, "single replica should have no affinity")
	})

	t.Run("multiple replicas have anti-affinity", func(t *testing.T) {
		deployment := controlledCloudflaredDeployment{
			config: CloudflaredConfig{Replicas: 3},
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
	aff1 := buildPodAntiAffinity("test", 2)
	aff2 := buildPodAntiAffinity("test", 2)

	assert.True(t, affinityEqual(nil, nil))
	assert.False(t, affinityEqual(aff1, nil))
	assert.False(t, affinityEqual(nil, aff1))
	assert.True(t, affinityEqual(aff1, aff2))
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
