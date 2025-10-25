package controller

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"slices"
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

func CreateOrUpdateControlledCloudflared(
	ctx context.Context,
	kubeClient client.Client,
	tunnelClient cloudflarecontroller.TunnelClientInterface,
	namespace string,
	protocol string,
	extraArgs []string,
) error {
	logger := log.FromContext(ctx)

	// List existing deployments with the specific label
	var list appsv1.DeploymentList
	if err := kubeClient.List(ctx, &list, &client.ListOptions{
		Namespace: namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			"strrl.dev/cloudflare-tunnel-ingress-controller": "controlled-cloudflared-connector",
		}),
	}); err != nil {
		return errors.Wrapf(err, "list controlled-cloudflared-connector in namespace %s", namespace)
	}

	if len(list.Items) > 0 {
		// Check if the existing deployment needs to be updated
		existingDeployment := &list.Items[0]
		desiredReplicas, err := getDesiredReplicas()
		if err != nil {
			return errors.Wrap(err, "get desired replicas")
		}

		needsUpdate := false
		if desiredReplicas >= 0 && existingDeployment.Spec.Replicas != nil && *existingDeployment.Spec.Replicas != desiredReplicas {
			needsUpdate = true
		}

		// Get token once for all checks
		token, err := tunnelClient.FetchTunnelToken(ctx)
		if err != nil {
			return errors.Wrap(err, "fetch tunnel token")
		}

		if len(existingDeployment.Spec.Template.Spec.Containers) > 0 {
			container := existingDeployment.Spec.Template.Spec.Containers[0]
			if container.Image != os.Getenv("CLOUDFLARED_IMAGE") {
				needsUpdate = true
			}
			if string(container.ImagePullPolicy) != os.Getenv("CLOUDFLARED_IMAGE_PULL_POLICY") {
				needsUpdate = true
			}

			// Check if command arguments have changed
			desiredCommand := getCloudflaredCommand(protocol, token, extraArgs)
			if !slices.Equal(container.Command, desiredCommand) {
				needsUpdate = true
			}

			// Check if resource requirements have changed
			desiredResources, err := getDesiredResources()
			if err != nil {
				return errors.Wrap(err, "get desired resources")
			}
			if os.Getenv("CLOUDFLARED_RESOURCES") != "" && !reflect.DeepEqual(container.Resources, desiredResources) {
				needsUpdate = true
			}
		}

		if needsUpdate {
			updatedDeployment := cloudflaredConnectDeploymentTemplating(protocol, token, namespace, desiredReplicas, extraArgs)
			existingDeployment.Spec = updatedDeployment.Spec
			if err := kubeClient.Update(ctx, existingDeployment); err != nil {
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

	deployment := cloudflaredConnectDeploymentTemplating(protocol, token, namespace, replicas, extraArgs)
	if err := kubeClient.Create(ctx, deployment); err != nil {
		return errors.Wrap(err, "create controlled-cloudflared-connector deployment")
	}
	logger.Info("Created controlled-cloudflared-connector deployment", "namespace", namespace)

	return nil
}

func cloudflaredConnectDeploymentTemplating(protocol string, token string, namespace string, replicas int32, extraArgs []string) *appsv1.Deployment {
	appName := "controlled-cloudflared-connector"

	// Use default values if environment variables are empty
	image := os.Getenv("CLOUDFLARED_IMAGE")
	if image == "" {
		image = "cloudflare/cloudflared:latest"
	}

	pullPolicy := os.Getenv("CLOUDFLARED_IMAGE_PULL_POLICY")
	if pullPolicy == "" {
		pullPolicy = "IfNotPresent"
	}

	// Ignore error, if any. If there's an error, resources will be empty and thus ignored by Kubernetes.
	resources, _ := getDesiredResources()

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: namespace,
			Labels: map[string]string{
				"app": appName,
				"strrl.dev/cloudflare-tunnel-ingress-controller": "controlled-cloudflared-connector",
			},
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
					Name: appName,
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
							Command:         getCloudflaredCommand(protocol, token, extraArgs),
							Resources:       resources,
						},
					},
					RestartPolicy: v1.RestartPolicyAlways,
				},
			},
		},
	}

	if replicas < 0 {
		deployment.Spec.Replicas = nil // Use Kubernetes default
	}

	return deployment
}

func getDesiredReplicas() (int32, error) {
	replicaCount := os.Getenv("CLOUDFLARED_REPLICA_COUNT")
	if replicaCount == "" {
		return -1, nil
	}
	replicas, err := strconv.ParseInt(replicaCount, 10, 32)
	if err != nil {
		return 0, errors.Wrap(err, "invalid replica count")
	}
	return int32(replicas), nil
}

func getCloudflaredCommand(protocol string, token string, extraArgs []string) []string {
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

func getDesiredResources() (v1.ResourceRequirements, error) {
	var desiredresources v1.ResourceRequirements

	resources := os.Getenv("CLOUDFLARED_RESOURCES")
	if resources == "" {
		return desiredresources, nil
	}

	if err := json.Unmarshal([]byte(resources), &desiredresources); err != nil {
		return desiredresources, errors.Wrap(err, "invalid resource requirements")
	}

	return desiredresources, nil
}
