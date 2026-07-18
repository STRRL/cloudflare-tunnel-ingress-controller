package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
)

func TestControlledCloudflaredDeploymentBuild(t *testing.T) {
	t.Setenv("CLOUDFLARED_IMAGE", "")
	t.Setenv("CLOUDFLARED_IMAGE_PULL_POLICY", "")

	deployment := controlledCloudflaredDeployment{
		protocol:           "quic",
		tokenSecretVersion: "42",
		namespace:          "controller-system",
		replicas:           2,
		extraArgs:          []string{"--post-quantum"},
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

func TestControlledCloudflaredDeploymentBuildUsesConfiguredImage(t *testing.T) {
	t.Setenv("CLOUDFLARED_IMAGE", "cloudflare/cloudflared:2026.7.0")
	t.Setenv("CLOUDFLARED_IMAGE_PULL_POLICY", "Always")

	deployment := controlledCloudflaredDeployment{}.build()
	container := deployment.Spec.Template.Spec.Containers[0]

	assert.Equal(t, "cloudflare/cloudflared:2026.7.0", container.Image)
	assert.Equal(t, v1.PullAlways, container.ImagePullPolicy)
}
