package controller

import (
	"context"
	"slices"

	cloudflarecontroller "github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/cloudflare-controller"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// CloudflaredConfig carries the fully resolved settings for the managed
// cloudflared connector deployment, configuration parsing stays in main.
type CloudflaredConfig struct {
	Image           string
	ImagePullPolicy string
	Replicas        int32
	Protocol        string
	ExtraArgs       []string
	PodAntiAffinity bool
}

func CreateOrUpdateControlledCloudflared(
	ctx context.Context,
	kubeClient client.Client,
	tunnelClient cloudflarecontroller.TunnelClientInterface,
	namespace string,
	config CloudflaredConfig,
) error {
	logger := log.FromContext(ctx)

	token, err := tunnelClient.FetchTunnelToken(ctx)
	if err != nil {
		return errors.Wrap(err, "fetch tunnel token")
	}

	tokenSecretVersion, err := createOrUpdateTunnelTokenSecret(ctx, kubeClient, namespace, token)
	if err != nil {
		return errors.Wrap(err, "create or update tunnel token secret")
	}

	list := appsv1.DeploymentList{}
	err = kubeClient.List(ctx, &list, &client.ListOptions{
		Namespace: namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			"strrl.dev/cloudflare-tunnel-ingress-controller": "controlled-cloudflared-connector",
		}),
	})
	if err != nil {
		return errors.Wrapf(err, "list controlled-cloudflared-connector in namespace %s", namespace)
	}

	if len(list.Items) > 0 {
		existingDeployment := &list.Items[0]

		needsUpdate := false
		if *existingDeployment.Spec.Replicas != config.Replicas {
			needsUpdate = true
		}

		if len(existingDeployment.Spec.Template.Spec.Containers) > 0 {
			container := &existingDeployment.Spec.Template.Spec.Containers[0]
			if container.Image != config.Image {
				needsUpdate = true
			}
			if string(container.ImagePullPolicy) != config.ImagePullPolicy {
				needsUpdate = true
			}

			desiredCommand := buildCloudflaredCommand(config.Protocol, config.ExtraArgs)
			if !slices.Equal(container.Command, desiredCommand) {
				needsUpdate = true
			}
		}
		if existingDeployment.Spec.Template.Annotations[tunnelTokenSecretVersionAnnotation] != tokenSecretVersion {
			needsUpdate = true
		}

		desiredAffinity := buildPodAntiAffinity("controlled-cloudflared-connector", config.PodAntiAffinity)
		if !equality.Semantic.DeepEqual(existingDeployment.Spec.Template.Spec.Affinity, desiredAffinity) {
			needsUpdate = true
		}

		if needsUpdate {
			updatedDeployment := controlledCloudflaredDeployment{
				config:             config,
				tokenSecretVersion: tokenSecretVersion,
				namespace:          namespace,
			}.build()
			existingDeployment.Spec = updatedDeployment.Spec
			err = kubeClient.Update(ctx, existingDeployment)
			if err != nil {
				return errors.Wrap(err, "update controlled-cloudflared-connector deployment")
			}
			logger.Info("Updated controlled-cloudflared-connector deployment", "namespace", namespace)
		}

		return nil
	}

	deployment := controlledCloudflaredDeployment{
		config:             config,
		tokenSecretVersion: tokenSecretVersion,
		namespace:          namespace,
	}.build()
	err = kubeClient.Create(ctx, deployment)
	if err != nil {
		return errors.Wrap(err, "create controlled-cloudflared-connector deployment")
	}
	logger.Info("Created controlled-cloudflared-connector deployment", "namespace", namespace)
	return nil
}

func createOrUpdateTunnelTokenSecret(
	ctx context.Context,
	kubeClient client.Client,
	namespace string,
	token string,
) (string, error) {
	logger := log.FromContext(ctx)

	existingSecret := &v1.Secret{}
	err := kubeClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      tunnelTokenSecretName,
	}, existingSecret)

	desiredSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tunnelTokenSecretName,
			Namespace: namespace,
			Labels: map[string]string{
				"strrl.dev/cloudflare-tunnel-ingress-controller": "controlled-cloudflared-connector",
			},
		},
		StringData: map[string]string{
			tunnelTokenSecretKey: token,
		},
	}

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return "", errors.Wrap(err, "get tunnel token secret")
		}
		err = kubeClient.Create(ctx, desiredSecret)
		if err != nil {
			return "", errors.Wrap(err, "create tunnel token secret")
		}
		logger.Info("Created tunnel token secret", "namespace", namespace)
		return desiredSecret.ResourceVersion, nil
	}

	if string(existingSecret.Data[tunnelTokenSecretKey]) == token {
		return existingSecret.ResourceVersion, nil
	}

	existingSecret.StringData = desiredSecret.StringData
	err = kubeClient.Update(ctx, existingSecret)
	if err != nil {
		return "", errors.Wrap(err, "update tunnel token secret")
	}
	logger.Info("Updated tunnel token secret", "namespace", namespace)
	return existingSecret.ResourceVersion, nil
}

const tunnelTokenSecretName = "controlled-cloudflared-token"
const tunnelTokenSecretKey = "tunnel-token"
const tunnelTokenSecretVersionAnnotation = "strrl.dev/cloudflare-tunnel-token-secret-version"

func buildCloudflaredCommand(protocol string, extraArgs []string) []string {
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

	// Add metrics and run subcommand
	// The tunnel token is provided via TUNNEL_TOKEN env var from a Kubernetes Secret
	command = append(command, "--metrics", "0.0.0.0:44483", "run")

	return command
}
