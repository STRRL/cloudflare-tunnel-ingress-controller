package controller

import (
	"context"
	"crypto/sha256"
	"fmt"
	"slices"
	"time"

	cloudflarecontroller "github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/cloudflare-controller"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const configHashAnnotation = "strrl.dev/cloudflared-config-hash"

// connectorAppName is the name shared by the managed cloudflared connector
// Deployment, its pods and the "app" label.
const connectorAppName = "controlled-cloudflared-connector"

// connectorManagedByLabelKey marks resources owned by this controller; its
// value is connectorAppName.
const connectorManagedByLabelKey = "strrl.dev/cloudflare-tunnel-ingress-controller"

// connectorLabels are the immutable labels identifying the managed cloudflared
// connector resources; they also serve as the Deployment selector.
func connectorLabels() map[string]string {
	return map[string]string{
		"app":                      connectorAppName,
		connectorManagedByLabelKey: connectorAppName,
	}
}

// CloudflaredConfig carries the fully resolved settings for the managed
// cloudflared connector deployment, configuration parsing stays in main.
type CloudflaredConfig struct {
	Image           string
	ImagePullPolicy string
	Replicas        int32
	Protocol        string
	ExtraArgs       []string
	// Customization holds the pod template customization loaded from the
	// deployment config file, nil means no customization.
	Customization *CloudflaredDeploymentConfig
	// CustomizationHash identifies the customization content, drift is
	// detected through the config hash annotation on the deployment.
	CustomizationHash string
	// Owner ties the lifecycle of the connector resources to the controller
	// Deployment: garbage collection removes them when the controller is
	// uninstalled. Nil (for example when running outside the cluster) leaves
	// the resources unowned.
	Owner *metav1.OwnerReference
	// TokenRefreshInterval is the minimum age the stored tunnel token must
	// reach before it is read from the Cloudflare API again. Zero re-reads on
	// every reconcile.
	TokenRefreshInterval time.Duration
}

// ResolveControllerOwnerReference builds the owner reference pointing at the
// controller Deployment the connector resources should be bound to.
func ResolveControllerOwnerReference(ctx context.Context, kubeClient client.Client, namespace string, name string) (*metav1.OwnerReference, error) {
	deployment := &appsv1.Deployment{}
	err := kubeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, deployment)
	if err != nil {
		return nil, errors.Wrapf(err, "get controller deployment %s/%s", namespace, name)
	}
	return &metav1.OwnerReference{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       deployment.Name,
		UID:        deployment.UID,
	}, nil
}

func hasOwnerReference(refs []metav1.OwnerReference, owner *metav1.OwnerReference) bool {
	for _, ref := range refs {
		if ref.UID == owner.UID {
			return true
		}
	}
	return false
}

// adoptConnectorResources appends the owner reference to connector resources
// created by older controller versions. It only talks to the Kubernetes API,
// so adoption succeeds even while Cloudflare is unreachable.
func adoptConnectorResources(ctx context.Context, kubeClient client.Client, namespace string, owner *metav1.OwnerReference) error {
	logger := log.FromContext(ctx)

	list := appsv1.DeploymentList{}
	err := kubeClient.List(ctx, &list, &client.ListOptions{
		Namespace: namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			connectorManagedByLabelKey: connectorAppName,
		}),
	})
	if err != nil {
		return errors.Wrapf(err, "list controlled-cloudflared-connector in namespace %s", namespace)
	}
	for i := range list.Items {
		deployment := &list.Items[i]
		if hasOwnerReference(deployment.OwnerReferences, owner) {
			continue
		}
		deployment.OwnerReferences = append(deployment.OwnerReferences, *owner)
		if err := kubeClient.Update(ctx, deployment); err != nil {
			return errors.Wrapf(err, "adopt connector deployment %s/%s", namespace, deployment.Name)
		}
		logger.Info("adopted connector deployment", "namespace", namespace, "name", deployment.Name)
	}

	secret := &v1.Secret{}
	err = kubeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: tunnelTokenSecretName}, secret)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "get tunnel token secret")
	}
	if !hasOwnerReference(secret.OwnerReferences, owner) {
		secret.OwnerReferences = append(secret.OwnerReferences, *owner)
		if err := kubeClient.Update(ctx, secret); err != nil {
			return errors.Wrap(err, "adopt tunnel token secret")
		}
		logger.Info("adopted tunnel token secret", "namespace", namespace, "name", tunnelTokenSecretName)
	}
	return nil
}

func CreateOrUpdateControlledCloudflared(
	ctx context.Context,
	kubeClient client.Client,
	tunnelClient cloudflarecontroller.TunnelClientInterface,
	namespace string,
	config CloudflaredConfig,
) error {
	logger := log.FromContext(ctx)

	// adopt pre-existing resources before any external call, lifecycle
	// ownership must not depend on Cloudflare availability
	if config.Owner != nil {
		if err := adoptConnectorResources(ctx, kubeClient, namespace, config.Owner); err != nil {
			return errors.Wrap(err, "adopt connector resources")
		}
	}

	tokenHash, err := reconcileTunnelTokenSecret(ctx, kubeClient, tunnelClient, namespace, config)
	if err != nil {
		return errors.Wrap(err, "reconcile tunnel token secret")
	}

	list := appsv1.DeploymentList{}
	err = kubeClient.List(ctx, &list, &client.ListOptions{
		Namespace: namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			connectorManagedByLabelKey: connectorAppName,
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
		if existingDeployment.Spec.Template.Annotations[tunnelTokenHashAnnotation] != tokenHash {
			needsUpdate = true
		}

		if existingDeployment.Annotations[configHashAnnotation] != config.CustomizationHash {
			needsUpdate = true
		}

		if needsUpdate {
			updatedDeployment := controlledCloudflaredDeployment{
				config:    config,
				tokenHash: tokenHash,
				namespace: namespace,
			}.build()
			existingDeployment.Spec = updatedDeployment.Spec
			if existingDeployment.Annotations == nil {
				existingDeployment.Annotations = map[string]string{}
			}
			if config.CustomizationHash != "" {
				existingDeployment.Annotations[configHashAnnotation] = config.CustomizationHash
			} else {
				delete(existingDeployment.Annotations, configHashAnnotation)
			}
			err = kubeClient.Update(ctx, existingDeployment)
			if err != nil {
				return errors.Wrap(err, "update controlled-cloudflared-connector deployment")
			}
			logger.Info("Updated controlled-cloudflared-connector deployment", "namespace", namespace)
		}

		return nil
	}

	deployment := controlledCloudflaredDeployment{
		config:    config,
		tokenHash: tokenHash,
		namespace: namespace,
	}.build()
	err = kubeClient.Create(ctx, deployment)
	if err != nil {
		return errors.Wrap(err, "create controlled-cloudflared-connector deployment")
	}
	logger.Info("Created controlled-cloudflared-connector deployment", "namespace", namespace)
	return nil
}

// reconcileTunnelTokenSecret keeps the Secret holding the tunnel token in sync
// and returns the hash of the token the connector pods should run with.
//
// The Cloudflare API is only consulted when the stored token is missing or
// older than config.TokenRefreshInterval.
func reconcileTunnelTokenSecret(
	ctx context.Context,
	kubeClient client.Client,
	tunnelClient cloudflarecontroller.TunnelClientInterface,
	namespace string,
	config CloudflaredConfig,
) (string, error) {
	logger := log.FromContext(ctx)

	existingSecret := &v1.Secret{}
	err := kubeClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      tunnelTokenSecretName,
	}, existingSecret)
	if err != nil && !apierrors.IsNotFound(err) {
		return "", errors.Wrap(err, "get tunnel token secret")
	}
	secretExists := err == nil

	storedToken := ""
	if secretExists {
		storedToken = string(existingSecret.Data[tunnelTokenSecretKey])
	}

	if storedToken != "" && !tunnelTokenNeedsRefresh(existingSecret, config.TokenRefreshInterval, time.Now()) {
		return tunnelTokenHash(storedToken), nil
	}

	token, err := tunnelClient.FetchTunnelToken(ctx)
	if err != nil {
		if storedToken != "" {
			logger.Error(err, "refresh tunnel token, continuing with the stored token", "namespace", namespace)
			return tunnelTokenHash(storedToken), nil
		}
		return "", errors.Wrap(err, "fetch tunnel token")
	}

	refreshedAt := time.Now().UTC().Format(time.RFC3339)

	if !secretExists {
		desiredSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tunnelTokenSecretName,
				Namespace: namespace,
				Labels: map[string]string{
					connectorManagedByLabelKey: connectorAppName,
				},
				Annotations: map[string]string{
					tunnelTokenRefreshedAtAnnotation: refreshedAt,
				},
			},
			StringData: map[string]string{
				tunnelTokenSecretKey: token,
			},
		}
		if config.Owner != nil {
			desiredSecret.OwnerReferences = []metav1.OwnerReference{*config.Owner}
		}
		if err := kubeClient.Create(ctx, desiredSecret); err != nil {
			return "", errors.Wrap(err, "create tunnel token secret")
		}
		logger.Info("Created tunnel token secret", "namespace", namespace)
		return tunnelTokenHash(token), nil
	}

	if existingSecret.Annotations == nil {
		existingSecret.Annotations = map[string]string{}
	}
	existingSecret.Annotations[tunnelTokenRefreshedAtAnnotation] = refreshedAt
	tokenChanged := storedToken != token
	if tokenChanged {
		existingSecret.StringData = map[string]string{
			tunnelTokenSecretKey: token,
		}
	}
	if err := kubeClient.Update(ctx, existingSecret); err != nil {
		return "", errors.Wrap(err, "update tunnel token secret")
	}
	if tokenChanged {
		logger.Info("Updated tunnel token secret", "namespace", namespace)
	}
	return tunnelTokenHash(token), nil
}

// tunnelTokenNeedsRefresh reports whether the token held by secret is old
// enough to be read from the Cloudflare API again. An interval of zero, or a
// missing or unparseable refresh stamp, refreshes.
func tunnelTokenNeedsRefresh(secret *v1.Secret, interval time.Duration, now time.Time) bool {
	if interval <= 0 {
		return true
	}
	refreshedAt, err := time.Parse(time.RFC3339, secret.Annotations[tunnelTokenRefreshedAtAnnotation])
	if err != nil {
		return true
	}
	return !now.Before(refreshedAt.Add(interval))
}

// tunnelTokenHash identifies a token value without exposing it through an
// annotation.
func tunnelTokenHash(token string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
}

const tunnelTokenSecretName = "controlled-cloudflared-token"
const tunnelTokenSecretKey = "tunnel-token"

// tunnelTokenHashAnnotation carries the hash of the tunnel token on the
// connector pod template so the pods roll when the token value changes. It
// tracks the token rather than the Secret resource version on purpose:
// unrelated writes to the Secret, such as adopting it or stamping a refresh,
// must not restart the connector.
const tunnelTokenHashAnnotation = "strrl.dev/cloudflare-tunnel-token-hash"

// tunnelTokenRefreshedAtAnnotation records on the token Secret when the token
// was last read from the Cloudflare API, in RFC3339.
const tunnelTokenRefreshedAtAnnotation = "strrl.dev/cloudflare-tunnel-token-refreshed-at"

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
