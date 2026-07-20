package controller

import (
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type controlledCloudflaredDeployment struct {
	config             CloudflaredConfig
	tokenSecretVersion string
	namespace          string
}

func (d controlledCloudflaredDeployment) build() *appsv1.Deployment {
	const appName = "controlled-cloudflared-connector"

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: d.namespace,
			Labels: map[string]string{
				"app": appName,
				"strrl.dev/cloudflare-tunnel-ingress-controller": "controlled-cloudflared-connector",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &d.config.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": appName,
					"strrl.dev/cloudflare-tunnel-ingress-controller": "controlled-cloudflared-connector",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: appName,
					Annotations: map[string]string{
						tunnelTokenSecretVersionAnnotation: d.tokenSecretVersion,
					},
					Labels: map[string]string{
						"app": appName,
						"strrl.dev/cloudflare-tunnel-ingress-controller": "controlled-cloudflared-connector",
					},
				},
				Spec: v1.PodSpec{
					Affinity: buildPodAntiAffinity(appName, d.config.PodAntiAffinity),
					Containers: []v1.Container{
						{
							Name:            appName,
							Image:           d.config.Image,
							ImagePullPolicy: v1.PullPolicy(d.config.ImagePullPolicy),
							Command:         buildCloudflaredCommand(d.config.Protocol, d.config.ExtraArgs),
							Env: []v1.EnvVar{
								{
									Name: "TUNNEL_TOKEN",
									ValueFrom: &v1.EnvVarSource{
										SecretKeyRef: &v1.SecretKeySelector{
											LocalObjectReference: v1.LocalObjectReference{
												Name: tunnelTokenSecretName,
											},
											Key: tunnelTokenSecretKey,
										},
									},
								},
							},
						},
					},
					RestartPolicy: v1.RestartPolicyAlways,
				},
			},
		},
	}
}

// buildPodAntiAffinity returns a pod anti-affinity that spreads pods across
// nodes, or nil when the feature is disabled. The hard scheduling requirement
// means replicas must not exceed the number of schedulable nodes.
func buildPodAntiAffinity(appName string, enabled bool) *v1.Affinity {
	if !enabled {
		return nil
	}
	return &v1.Affinity{
		PodAntiAffinity: &v1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": appName,
						},
					},
					TopologyKey: "kubernetes.io/hostname",
				},
			},
		},
	}
}
