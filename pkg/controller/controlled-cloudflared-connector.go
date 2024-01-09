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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateControlledCloudflaredIfNotExist(
	ctx context.Context,
	kubeClient client.Client,
	tunnelClient *cloudflarecontroller.TunnelClient,
	namespace string,
) error {
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
		return nil
	}

	token, err := tunnelClient.FetchTunnelToken(ctx)
	if err != nil {
		return errors.Wrap(err, "fetch tunnel token")
	}

	controllerPod := v1.Pod{}
	err = kubeClient.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      os.Getenv("POD_NAME"),
	}, &controllerPod)
	if err != nil {
		return errors.Wrap(err, "get controller pod")
	}

	var controllerVolumeMounts []v1.VolumeMount
	for _, container := range controllerPod.Spec.Containers {
		if container.Name == "cloudflare-tunnel-ingress-controller" {
			controllerVolumeMounts = container.VolumeMounts
		}
	}

	replicas, err := strconv.ParseInt(os.Getenv("CLOUDFLARED_REPLICA_COUNT"), 10, 32)
	if err != nil {
		return errors.Wrap(err, "invalid replica count")
	}

	deployment := cloudflaredConnectDeploymentTemplating(
		token,
		namespace,
		int32(replicas),
		controllerPod.Spec.Volumes,
		controllerVolumeMounts,
	)

	err = kubeClient.Create(ctx, deployment)
	if err != nil {
		return errors.Wrap(err, "create controlled-cloudflared-connector deployment")
	}
	return nil
}

func cloudflaredConnectDeploymentTemplating(
	token string,
	namespace string,
	replicas int32,
	extraVolumes []v1.Volume,
	extraVolumeMounts []v1.VolumeMount,
) *appsv1.Deployment {
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
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: appName,
					Labels: map[string]string{
						"app": appName,
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
							VolumeMounts: extraVolumeMounts,
						},
					},
					RestartPolicy: v1.RestartPolicyAlways,
					Volumes:       extraVolumes,
				},
			},
		},
	}
}
