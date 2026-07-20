package controller

import (
	"context"
	"slices"
	"strconv"

	cloudflarecontroller "github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/cloudflare-controller"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
		desiredReplicas, err := getDesiredReplicas()
		if err != nil {
			return errors.Wrap(err, "get desired replicas")
		}

		needsUpdate := false
		if *existingDeployment.Spec.Replicas != desiredReplicas {
			needsUpdate = true
		}

		if len(existingDeployment.Spec.Template.Spec.Containers) > 0 {
			container := &existingDeployment.Spec.Template.Spec.Containers[0]
			if container.Image != cloudflaredImage() {
				needsUpdate = true
			}
			if string(container.ImagePullPolicy) != cloudflaredImagePullPolicy() {
				needsUpdate = true
			}

			desiredCommand := buildCloudflaredCommand(protocol, extraArgs)
			if !slices.Equal(container.Command, desiredCommand) {
				needsUpdate = true
			}
		}
		if existingDeployment.Spec.Template.Annotations[tunnelTokenSecretVersionAnnotation] != tokenSecretVersion {
			needsUpdate = true
		}

		if needsUpdate {
			updatedDeployment := controlledCloudflaredDeployment{
				protocol:           protocol,
				tokenSecretVersion: tokenSecretVersion,
				namespace:          namespace,
				replicas:           desiredReplicas,
				extraArgs:          extraArgs,
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

	replicas, err := getDesiredReplicas()
	if err != nil {
		return errors.Wrap(err, "get desired replicas")
	}

	deployment := controlledCloudflaredDeployment{
		protocol:           protocol,
		tokenSecretVersion: tokenSecretVersion,
		namespace:          namespace,
		replicas:           replicas,
		extraArgs:          extraArgs,
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

func getDesiredReplicas() (int32, error) {
	raw := viper.GetString("cloudflared-replica-count")
	if raw == "" {
		return 1, nil
	}
	replicas, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, errors.Wrap(err, "invalid replica count")
	}
	return int32(replicas), nil
}

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
