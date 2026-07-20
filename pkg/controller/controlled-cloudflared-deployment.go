package controller

import (
	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func cloudflaredImage() string {
	if image := viper.GetString("cloudflared-image"); image != "" {
		return image
	}
	return "cloudflare/cloudflared:latest"
}

func cloudflaredImagePullPolicy() string {
	if policy := viper.GetString("cloudflared-image-pull-policy"); policy != "" {
		return policy
	}
	return "IfNotPresent"
}

type controlledCloudflaredDeployment struct {
	protocol           string
	tokenSecretVersion string
	namespace          string
	replicas           int32
	extraArgs          []string
}

func (d controlledCloudflaredDeployment) build() *appsv1.Deployment {
	const appName = "controlled-cloudflared-connector"

	image := cloudflaredImage()
	pullPolicy := cloudflaredImagePullPolicy()

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
			Replicas: &d.replicas,
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
					Containers: []v1.Container{
						{
							Name:            appName,
							Image:           image,
							ImagePullPolicy: v1.PullPolicy(pullPolicy),
							Command:         buildCloudflaredCommand(d.protocol, d.extraArgs),
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
