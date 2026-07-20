package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestControlledCloudflaredDeploymentBuildCustomization(t *testing.T) {
	t.Run("no customization keeps a plain pod spec", func(t *testing.T) {
		deployment := controlledCloudflaredDeployment{
			config: CloudflaredConfig{Replicas: 3},
		}.build()

		podSpec := deployment.Spec.Template.Spec
		assert.Nil(t, podSpec.Affinity)
		assert.Nil(t, podSpec.SecurityContext)
		assert.Empty(t, podSpec.NodeSelector)
		assert.Empty(t, deployment.Annotations)
	})

	t.Run("customization is applied to the pod template", func(t *testing.T) {
		customization := &CloudflaredDeploymentConfig{
			Resources: &v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("100m"),
				},
			},
			PodLabels:      map[string]string{"team": "platform"},
			PodAnnotations: map[string]string{"prometheus.io/scrape": "true"},
			NodeSelector:   map[string]string{"kubernetes.io/os": "linux"},
			Affinity: &v1.Affinity{
				PodAntiAffinity: &v1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
						{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"app": "controlled-cloudflared-connector"},
							},
							TopologyKey: "kubernetes.io/hostname",
						},
					},
				},
			},
			PriorityClassName: "high-priority",
		}

		deployment := controlledCloudflaredDeployment{
			config: CloudflaredConfig{
				Replicas:          2,
				Customization:     customization,
				CustomizationHash: "abc123",
			},
			tokenSecretVersion: "42",
		}.build()

		podSpec := deployment.Spec.Template.Spec
		assert.Equal(t, "100m", podSpec.Containers[0].Resources.Requests.Cpu().String())
		assert.Equal(t, map[string]string{"kubernetes.io/os": "linux"}, podSpec.NodeSelector)
		require.NotNil(t, podSpec.Affinity)
		require.NotNil(t, podSpec.Affinity.PodAntiAffinity)
		assert.Equal(t, "high-priority", podSpec.PriorityClassName)
		assert.Equal(t, "abc123", deployment.Annotations[configHashAnnotation])

		labels := deployment.Spec.Template.Labels
		assert.Equal(t, "platform", labels["team"])
		annotations := deployment.Spec.Template.Annotations
		assert.Equal(t, "true", annotations["prometheus.io/scrape"])
		assert.Equal(t, "42", annotations[tunnelTokenSecretVersionAnnotation])
	})

	t.Run("customization cannot override selector labels or token annotation", func(t *testing.T) {
		deployment := controlledCloudflaredDeployment{
			config: CloudflaredConfig{
				Replicas: 1,
				Customization: &CloudflaredDeploymentConfig{
					PodLabels: map[string]string{
						"app": "hijacked",
						"strrl.dev/cloudflare-tunnel-ingress-controller": "hijacked",
					},
					PodAnnotations: map[string]string{
						tunnelTokenSecretVersionAnnotation: "hijacked",
					},
				},
			},
			tokenSecretVersion: "42",
		}.build()

		labels := deployment.Spec.Template.Labels
		assert.Equal(t, "controlled-cloudflared-connector", labels["app"])
		assert.Equal(t, "controlled-cloudflared-connector", labels["strrl.dev/cloudflare-tunnel-ingress-controller"])
		assert.Equal(t, "42", deployment.Spec.Template.Annotations[tunnelTokenSecretVersionAnnotation])
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
