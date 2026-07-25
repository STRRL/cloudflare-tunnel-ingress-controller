package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/cucumber/godog"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const (
	uninstallReleaseName     = "cf-ic-e2e-uninstall"
	uninstallNamespace       = "cf-ic-e2e-uninstall"
	uninstallIngressClass    = "cloudflare-tunnel-uninstall"
	uninstallControllerClass = "strrl.dev/cloudflare-tunnel-ingress-controller-uninstall-e2e"
	connectorDeploymentName  = "controlled-cloudflared-connector"
	connectorTokenSecret     = "controlled-cloudflared-token"
)

// registerUninstallCleanupSteps wires the uninstall cleanup scenario. It runs
// a dedicated release with its own tunnel and ingress class so the shared
// suite release stays untouched.
func registerUninstallCleanupSteps(ctx *godog.ScenarioContext) {
	ctx.After(func(ctx context.Context, sc *godog.Scenario, scErr error) (context.Context, error) {
		w := worldFromContext(ctx)
		if w == nil || w.uninstallTunnelName == "" {
			return ctx, nil
		}

		// best effort cleanup for aborted scenarios
		if !w.uninstallReleaseRemoved {
			_ = helmUninstall(context.Background(), uninstallReleaseName, uninstallNamespace)
		}
		// the tunnel intentionally survives uninstall, remove the scenario
		// scoped one so test tunnels do not pile up on the account
		if api, err := newCloudflareAPI(); err == nil {
			_ = deleteTunnelByName(context.Background(), api, w.uninstallTunnelName)
		}
		_ = kubeClient.CoreV1().Namespaces().Delete(context.Background(), uninstallNamespace, metav1.DeleteOptions{})

		return ctx, nil
	})

	ctx.Step(`^a dedicated controller release with its own tunnel is installed$`, aDedicatedControllerReleaseIsInstalled)
	ctx.Step(`^the dedicated connector deployment eventually becomes available$`, theDedicatedConnectorBecomesAvailable)
	ctx.Step(`^the dedicated release is uninstalled$`, theDedicatedReleaseIsUninstalled)
	ctx.Step(`^the dedicated connector resources are gone from the cluster$`, theDedicatedConnectorResourcesAreGone)
	ctx.Step(`^the dedicated tunnel still exists on Cloudflare$`, theDedicatedTunnelStillExists)
}

func aDedicatedControllerReleaseIsInstalled(ctx context.Context) error {
	w := worldFromContext(ctx)
	w.uninstallTunnelName = fmt.Sprintf("cf-ic-e2e-uninstall-%d", time.Now().UnixNano())

	values := controllerHelmValues{}
	values.Cloudflare.AccountID = os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	values.Cloudflare.TunnelName = w.uninstallTunnelName
	values.Cloudflare.APIToken = os.Getenv("CLOUDFLARE_API_TOKEN")
	values.Image.Repository = controllerImageRef.repository
	values.Image.Tag = controllerImageRef.tag
	values.Image.PullPolicy = "IfNotPresent"
	values.ClusterDomain = e2eClusterDomain
	values.IngressClassName = uninstallIngressClass
	values.ControllerClassValue = uninstallControllerClass

	installCtx, cancel := context.WithTimeout(suiteCtx, 10*time.Minute)
	defer cancel()
	if err := helmUpgradeInstall(installCtx, kubeconfigPath, uninstallReleaseName, uninstallNamespace, values); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(logOut, "dedicated release installed with tunnel %s\n", w.uninstallTunnelName)
	return nil
}

func theDedicatedConnectorBecomesAvailable(ctx context.Context) error {
	return waitFor("dedicated connector deployment available", 10*time.Minute, 10*time.Second, func() error {
		deployment, err := kubeClient.AppsV1().Deployments(uninstallNamespace).Get(context.Background(), connectorDeploymentName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if !isDeploymentAvailable(*deployment) {
			return fmt.Errorf("connector deployment has %d available replicas", deployment.Status.AvailableReplicas)
		}
		return nil
	})
}

func theDedicatedReleaseIsUninstalled(ctx context.Context) error {
	w := worldFromContext(ctx)

	uninstallCtx, cancel := context.WithTimeout(suiteCtx, 10*time.Minute)
	defer cancel()
	if err := helmUninstall(uninstallCtx, uninstallReleaseName, uninstallNamespace); err != nil {
		return err
	}
	w.uninstallReleaseRemoved = true
	return nil
}

func theDedicatedConnectorResourcesAreGone(ctx context.Context) error {
	// garbage collection removes the connector resources shortly after the
	// controller deployment is deleted, the poll absorbs that propagation
	return waitFor("dedicated connector resources deleted", 2*time.Minute, 5*time.Second, func() error {
		_, err := kubeClient.AppsV1().Deployments(uninstallNamespace).Get(context.Background(), connectorDeploymentName, metav1.GetOptions{})
		if err == nil {
			return fmt.Errorf("connector deployment still present")
		}
		if !apierrors.IsNotFound(err) {
			return err
		}
		_, err = kubeClient.CoreV1().Secrets(uninstallNamespace).Get(context.Background(), connectorTokenSecret, metav1.GetOptions{})
		if err == nil {
			return fmt.Errorf("tunnel token secret still present")
		}
		if !apierrors.IsNotFound(err) {
			return err
		}
		_, err = kubeClient.NetworkingV1().IngressClasses().Get(context.Background(), uninstallIngressClass, metav1.GetOptions{})
		if err == nil {
			return fmt.Errorf("ingress class still present")
		}
		if !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	})
}

func theDedicatedTunnelStillExists(ctx context.Context) error {
	w := worldFromContext(ctx)

	api, err := newCloudflareAPI()
	if err != nil {
		return err
	}
	accountID := os.Getenv("CLOUDFLARE_ACCOUNT_ID")

	tunnels, _, err := api.ListTunnels(context.Background(), cloudflare.ResourceIdentifier(accountID), cloudflare.TunnelListParams{
		Name:      w.uninstallTunnelName,
		IsDeleted: ptr.To(false),
	})
	if err != nil {
		return fmt.Errorf("list tunnels: %w", err)
	}
	if len(tunnels) == 0 {
		return fmt.Errorf("tunnel %s no longer exists on cloudflare, uninstall must not touch external resources", w.uninstallTunnelName)
	}
	return nil
}

func helmUninstall(ctx context.Context, releaseName string, namespace string) error {
	cmd := exec.CommandContext(ctx, "helm", "uninstall", releaseName, "--namespace", namespace, "--wait")
	cmd.Stdout = logOut
	cmd.Stderr = logOut
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("helm uninstall %s: %w", releaseName, err)
	}
	return nil
}

// deleteTunnelByName force removes the scenario scoped tunnel, uninstall
// intentionally keeps it so the test cleans it up itself.
func deleteTunnelByName(ctx context.Context, api *cloudflare.API, tunnelName string) error {
	accountID := os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	rc := cloudflare.ResourceIdentifier(accountID)
	tunnels, _, err := api.ListTunnels(ctx, rc, cloudflare.TunnelListParams{
		Name:      tunnelName,
		IsDeleted: ptr.To(false),
	})
	if err != nil {
		return err
	}
	for _, tunnel := range tunnels {
		_ = api.CleanupTunnelConnections(ctx, rc, tunnel.ID)
		if err := api.DeleteTunnel(ctx, rc, tunnel.ID); err != nil {
			return err
		}
	}
	return nil
}
