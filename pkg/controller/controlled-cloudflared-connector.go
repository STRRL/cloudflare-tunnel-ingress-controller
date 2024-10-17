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

func CreateOrUpdateControlledCloudflared(
	ctx context.Context,
	kubeClient client.Client,
	tunnelClient cloudflarecontroller.TunnelClientInterface,
	namespace string,
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

	if len(list.Items) > 0 {
		// Check if the existing deployment needs to be updated
		existingDeployment := &list.Items[0]
		desiredReplicas, err := strconv.ParseInt(os.Getenv("CLOUDFLARED_REPLICA_COUNT"), 10, 32)
		if err != nil {
			return errors.Wrap(err, "invalid replica count")
		}

		needsUpdate := false
		if *existingDeployment.Spec.Replicas != int32(desiredReplicas) {
			needsUpdate = true
		}

		if len(existingDeployment.Spec.Template.Spec.Containers) > 0 {
			container := &existingDeployment.Spec.Template.Spec.Containers[0]
			if container.Image != os.Getenv("CLOUDFLARED_IMAGE") {
				needsUpdate = true
			}
			if string(container.ImagePullPolicy) != os.Getenv("CLOUDFLARED_IMAGE_PULL_POLICY") {
				needsUpdate = true
			}
		}

		if needsUpdate {
			token, err := tunnelClient.FetchTunnelToken(ctx)
			if err != nil {
				return errors.Wrap(err, "fetch tunnel token")
			}

			updatedDeployment := cloudflaredConnectDeploymentTemplating(token, namespace, int32(desiredReplicas))
			existingDeployment.Spec = updatedDeployment.Spec
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

	replicas, err := strconv.ParseInt(os.Getenv("CLOUDFLARED_REPLICA_COUNT"), 10, 32)
	if err != nil {
		return errors.Wrap(err, "invalid replica count")
	}

	deployment := cloudflaredConnectDeploymentTemplating(token, namespace, int32(replicas))
	err = kubeClient.Create(ctx, deployment)
	if err != nil {
		return errors.Wrap(err, "create controlled-cloudflared-connector deployment")
	}
	logger.Info("Created controlled-cloudflared-connector deployment", "namespace", namespace)
	return nil
}

func cloudflaredConnectDeploymentTemplating(token string, namespace string, replicas int32) *appsv1.Deployment {
	appName := "controlled-cloudflared-connector"
	image := os.Getenv("CLOUDFLARED_IMAGE")
	pullPolicy := os.Getenv("CLOUDFLARED_IMAGE_PULL_POLICY")

	return &appsv1.Deployment{
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
							Command: []string{
								"cloudflared",
								"--no-autoupdate",
								"tunnel",
								"--metrics",
								"0.0.0.0:44483",
								"run",
								"--token",
								token,
							},
						},
					},
					RestartPolicy: v1.RestartPolicyAlways,
				},
			},
		},
	}
}
