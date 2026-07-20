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

	customization := d.config.Customization
	if customization == nil {
		customization = &CloudflaredDeploymentConfig{}
	}

	// User labels and annotations go first, the controller owned keys are set
	// afterwards so a customization can never override them.
	podLabels := map[string]string{}
	for k, v := range customization.PodLabels {
		podLabels[k] = v
	}
	podLabels["app"] = appName
	podLabels["strrl.dev/cloudflare-tunnel-ingress-controller"] = "controlled-cloudflared-connector"

	podAnnotations := map[string]string{}
	for k, v := range customization.PodAnnotations {
		podAnnotations[k] = v
	}
	podAnnotations[tunnelTokenSecretVersionAnnotation] = d.tokenSecretVersion

	deploymentAnnotations := map[string]string{}
	if d.config.CustomizationHash != "" {
		deploymentAnnotations[configHashAnnotation] = d.config.CustomizationHash
	}

	container := v1.Container{
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
	}

	if customization.Resources != nil {
		container.Resources = *customization.Resources
	}
	if customization.SecurityContext != nil {
		container.SecurityContext = customization.SecurityContext
	}
	if len(customization.VolumeMounts) > 0 {
		container.VolumeMounts = customization.VolumeMounts
	}
	if customization.Probes != nil {
		container.LivenessProbe = customization.Probes.Liveness
		container.ReadinessProbe = customization.Probes.Readiness
		container.StartupProbe = customization.Probes.Startup
	}

	podSpec := v1.PodSpec{
		Containers:    []v1.Container{container},
		RestartPolicy: v1.RestartPolicyAlways,
	}

	if customization.PodSecurityContext != nil {
		podSpec.SecurityContext = customization.PodSecurityContext
	}
	if len(customization.NodeSelector) > 0 {
		podSpec.NodeSelector = customization.NodeSelector
	}
	if len(customization.Tolerations) > 0 {
		podSpec.Tolerations = customization.Tolerations
	}
	if customization.Affinity != nil {
		podSpec.Affinity = customization.Affinity
	}
	if len(customization.TopologySpreadConstraints) > 0 {
		podSpec.TopologySpreadConstraints = customization.TopologySpreadConstraints
	}
	if customization.PriorityClassName != "" {
		podSpec.PriorityClassName = customization.PriorityClassName
	}
	if len(customization.Volumes) > 0 {
		podSpec.Volumes = customization.Volumes
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: d.namespace,
			Labels: map[string]string{
				"app": appName,
				"strrl.dev/cloudflare-tunnel-ingress-controller": "controlled-cloudflared-connector",
			},
			Annotations: deploymentAnnotations,
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
					Name:        appName,
					Annotations: podAnnotations,
					Labels:      podLabels,
				},
				Spec: podSpec,
			},
		},
	}
}
