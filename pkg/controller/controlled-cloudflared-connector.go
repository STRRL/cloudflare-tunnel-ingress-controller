package controller

import (
	"context"
	"os"
	"strconv"

	cloudflarecontroller "github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/cloudflare-controller"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const configHashAnnotation = "strrl.dev/cloudflared-config-hash"

func CreateOrUpdateControlledCloudflared(
	ctx context.Context,
	kubeClient client.Client,
	tunnelClient cloudflarecontroller.TunnelClientInterface,
	namespace string,
	protocol string,
	extraArgs []string,
	deploymentConfig *CloudflaredDeploymentConfig,
	configHash string,
) error {
	logger := log.FromContext(ctx)
	list := appsv1.DeploymentList{}
	err := kubeClient.List(ctx, &list, &client.ListOptions{
		Namespace: namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			"strrl.dev/cloudflare-tunnel-ingress-controller": "controlled-cloudflared-connector",
		}),
	})
	if err != nil {
		return errors.Wrapf(err, "list controlled-cloudflared-connector in namespace %s", namespace)
	}

	if deploymentConfig == nil {
		deploymentConfig = &CloudflaredDeploymentConfig{}
	}

	if len(list.Items) > 0 {
		existingDeployment := &list.Items[0]
		desiredReplicas, err := getDesiredReplicas()
		if err != nil {
			return errors.Wrap(err, "get desired replicas")
		}

		needsUpdate := false
		if *existingDeployment.Spec.Replicas != desiredReplicas {
			needsUpdate = true
		}

		token, err := tunnelClient.FetchTunnelToken(ctx)
		if err != nil {
			return errors.Wrap(err, "fetch tunnel token")
		}

		if len(existingDeployment.Spec.Template.Spec.Containers) > 0 {
			container := &existingDeployment.Spec.Template.Spec.Containers[0]
			if container.Image != os.Getenv("CLOUDFLARED_IMAGE") {
				needsUpdate = true
			}
			if string(container.ImagePullPolicy) != os.Getenv("CLOUDFLARED_IMAGE_PULL_POLICY") {
				needsUpdate = true
			}

			desiredCommand := buildCloudflaredCommand(protocol, token, extraArgs)
			if !slicesEqual(container.Command, desiredCommand) {
				needsUpdate = true
			}
		}

		// Check if config hash has changed
		existingHash := existingDeployment.Annotations[configHashAnnotation]
		if existingHash != configHash {
			needsUpdate = true
		}

		if needsUpdate {
			updatedDeployment := cloudflaredConnectDeploymentTemplating(protocol, token, namespace, desiredReplicas, extraArgs, deploymentConfig, configHash)
			existingDeployment.Spec = updatedDeployment.Spec
			if existingDeployment.Annotations == nil {
				existingDeployment.Annotations = make(map[string]string)
			}
			existingDeployment.Annotations[configHashAnnotation] = configHash
			err = kubeClient.Update(ctx, existingDeployment)
			if err != nil {
				return errors.Wrap(err, "update controlled-cloudflared-connector deployment")
			}
			logger.Info("Updated controlled-cloudflared-connector deployment", "namespace", namespace)
		}

		return nil
	}

	token, err := tunnelClient.FetchTunnelToken(ctx)
	if err != nil {
		return errors.Wrap(err, "fetch tunnel token")
	}

	replicas, err := getDesiredReplicas()
	if err != nil {
		return errors.Wrap(err, "get desired replicas")
	}

	deployment := cloudflaredConnectDeploymentTemplating(protocol, token, namespace, replicas, extraArgs, deploymentConfig, configHash)
	err = kubeClient.Create(ctx, deployment)
	if err != nil {
		return errors.Wrap(err, "create controlled-cloudflared-connector deployment")
	}
	logger.Info("Created controlled-cloudflared-connector deployment", "namespace", namespace)
	return nil
}

func cloudflaredConnectDeploymentTemplating(
	protocol string,
	token string,
	namespace string,
	replicas int32,
	extraArgs []string,
	config *CloudflaredDeploymentConfig,
	configHash string,
) *appsv1.Deployment {
	appName := "controlled-cloudflared-connector"

	image := os.Getenv("CLOUDFLARED_IMAGE")
	if image == "" {
		image = "cloudflare/cloudflared:latest"
	}

	pullPolicy := os.Getenv("CLOUDFLARED_IMAGE_PULL_POLICY")
	if pullPolicy == "" {
		pullPolicy = "IfNotPresent"
	}

	if config == nil {
		config = &CloudflaredDeploymentConfig{}
	}

	// Build pod labels: user-defined labels first, then required selector labels
	// Selector labels are set last to prevent user overrides from breaking the Deployment
	podLabels := map[string]string{}
	for k, v := range config.PodLabels {
		podLabels[k] = v
	}
	podLabels["app"] = appName
	podLabels["strrl.dev/cloudflare-tunnel-ingress-controller"] = "controlled-cloudflared-connector"

	// Build deployment annotations
	deploymentAnnotations := map[string]string{}
	if configHash != "" {
		deploymentAnnotations[configHashAnnotation] = configHash
	}

	// Build container
	container := v1.Container{
		Name:            appName,
		Image:           image,
		ImagePullPolicy: v1.PullPolicy(pullPolicy),
		Command:         buildCloudflaredCommand(protocol, token, extraArgs),
	}

	if config.Resources != nil {
		container.Resources = *config.Resources
	}
	if config.SecurityContext != nil {
		container.SecurityContext = config.SecurityContext
	}
	if len(config.VolumeMounts) > 0 {
		container.VolumeMounts = config.VolumeMounts
	}
	if config.Probes != nil {
		if config.Probes.Liveness != nil {
			container.LivenessProbe = config.Probes.Liveness
		}
		if config.Probes.Readiness != nil {
			container.ReadinessProbe = config.Probes.Readiness
		}
		if config.Probes.Startup != nil {
			container.StartupProbe = config.Probes.Startup
		}
	}

	// Build pod spec
	podSpec := v1.PodSpec{
		Containers:    []v1.Container{container},
		RestartPolicy: v1.RestartPolicyAlways,
	}

	if config.PodSecurityContext != nil {
		podSpec.SecurityContext = config.PodSecurityContext
	}
	if len(config.NodeSelector) > 0 {
		podSpec.NodeSelector = config.NodeSelector
	}
	if len(config.Tolerations) > 0 {
		podSpec.Tolerations = config.Tolerations
	}
	if config.Affinity != nil {
		podSpec.Affinity = config.Affinity
	}
	if len(config.TopologySpreadConstraints) > 0 {
		podSpec.TopologySpreadConstraints = config.TopologySpreadConstraints
	}
	if config.PriorityClassName != "" {
		podSpec.PriorityClassName = config.PriorityClassName
	}
	if len(config.Volumes) > 0 {
		podSpec.Volumes = config.Volumes
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: namespace,
			Labels: map[string]string{
				"app": appName,
				"strrl.dev/cloudflare-tunnel-ingress-controller": "controlled-cloudflared-connector",
			},
			Annotations: deploymentAnnotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": appName,
					"strrl.dev/cloudflare-tunnel-ingress-controller": "controlled-cloudflared-connector",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:        appName,
					Labels:      podLabels,
					Annotations: config.PodAnnotations,
				},
				Spec: podSpec,
			},
		},
	}
}

func getDesiredReplicas() (int32, error) {
	replicaCount := os.Getenv("CLOUDFLARED_REPLICA_COUNT")
	if replicaCount == "" {
		return 1, nil
	}
	replicas, err := strconv.ParseInt(replicaCount, 10, 32)
	if err != nil {
		return 0, errors.Wrap(err, "invalid replica count")
	}
	return int32(replicas), nil
}

func buildCloudflaredCommand(protocol string, token string, extraArgs []string) []string {
	command := []string{
		"cloudflared",
		"--protocol",
		protocol,
		"--no-autoupdate",
		"tunnel",
	}

	// Add all extra arguments between "tunnel" and "run"
	if len(extraArgs) > 0 {
		command = append(command, extraArgs...)
	}

	// Add metrics, run subcommand and token
	command = append(command, "--metrics", "0.0.0.0:44483", "run", "--token", token)

	return command
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
