package e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("Happy Path", func() {
	const (
		controllerNamespace   = "cloudflare-tunnel-ingress-controller"
		controllerReleaseName = "cf-ic-e2e"
		dashboardNamespace    = "kubernetes-dashboard"
		dashboardIngressName  = "dashboard-via-cloudflare"
	)

	It("exposes the Kubernetes dashboard via Cloudflare Tunnel", func() {
		By("ensuring the minikube node becomes Ready")
		waitFor("nodes ready", 10*time.Minute, 10*time.Second, func() error {
			nodes, err := kubeClient.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			if err != nil {
				return err
			}
			if len(nodes.Items) == 0 {
				return fmt.Errorf("no nodes found")
			}
			for _, node := range nodes.Items {
				if !isNodeReady(node) {
					return fmt.Errorf("node %s not Ready", node.Name)
				}
			}
			return nil
		})

		By("loading the controller image into the minikube profile")
		loadCtx, cancelLoad := context.WithTimeout(suiteCtx, 5*time.Minute)
		Expect(loadImageIntoMinikube(loadCtx, minikubeProfile, controllerImage)).To(Succeed())
		cancelLoad()

		By("installing the controller Helm chart")
		values := controllerHelmValues{}
		values.Cloudflare.AccountID = os.Getenv("CLOUDFLARE_ACCOUNT_ID")
		values.Cloudflare.TunnelName = os.Getenv("CLOUDFLARE_TUNNEL_NAME")
		values.Cloudflare.APIToken = os.Getenv("CLOUDFLARE_API_TOKEN")
		values.Image.Repository = controllerImageRef.repository
		values.Image.Tag = controllerImageRef.tag
		values.Image.PullPolicy = "IfNotPresent"

		helmCtx, cancelHelm := context.WithTimeout(suiteCtx, 10*time.Minute)
		Expect(helmUpgradeInstall(helmCtx, kubeconfigPath, controllerReleaseName, controllerNamespace, values)).To(Succeed())
		cancelHelm()

		DeferCleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if err := helmUninstall(cleanupCtx, kubeconfigPath, controllerReleaseName, controllerNamespace); err != nil {
				GinkgoWriter.Write([]byte(fmt.Sprintf("warning: failed to uninstall helm release %s: %v\n", controllerReleaseName, err)))
			}
		})

		By("waiting for the controller deployment to become Available")
		waitFor("controller deployment ready", 10*time.Minute, 10*time.Second, func() error {
			deployments, err := kubeClient.AppsV1().Deployments(controllerNamespace).List(context.Background(), metav1.ListOptions{
				LabelSelector: fmt.Sprintf("app.kubernetes.io/instance=%s", controllerReleaseName),
			})
			if err != nil {
				return err
			}
			if len(deployments.Items) == 0 {
				return fmt.Errorf("controller deployment not created")
			}
			for _, deployment := range deployments.Items {
				if !isDeploymentAvailable(deployment) {
					return fmt.Errorf("deployment %s has %d available replicas", deployment.Name, deployment.Status.AvailableReplicas)
				}
			}
			return nil
		})

		By("enabling dashboard related addons")
		addons := []string{"dashboard", "metrics-server"}
		for _, addon := range addons {
			enableCtx, cancelEnable := context.WithTimeout(suiteCtx, 5*time.Minute)
			Expect(enableMinikubeAddon(enableCtx, minikubeProfile, addon)).To(Succeed(), fmt.Sprintf("enable addon %s", addon))
			cancelEnable()
		}

		DeferCleanup(func() {
			disableCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			for _, addon := range addons {
				if err := disableMinikubeAddon(disableCtx, minikubeProfile, addon); err != nil {
					GinkgoWriter.Write([]byte(fmt.Sprintf("warning: failed to disable addon %s: %v\n", addon, err)))
				}
			}
		})

		By("waiting for the dashboard deployment to be Ready")
		waitFor("dashboard deployment ready", 10*time.Minute, 10*time.Second, func() error {
			deployment, err := kubeClient.AppsV1().Deployments(dashboardNamespace).Get(context.Background(), "kubernetes-dashboard", metav1.GetOptions{})
			if err != nil {
				return err
			}
			if !isDeploymentAvailable(*deployment) {
				return fmt.Errorf("dashboard deployment has %d available replicas", deployment.Status.AvailableReplicas)
			}
			return nil
		})

		var dashboardService *corev1.Service
		By("waiting for the dashboard service to expose a port")
		waitFor("dashboard service ports", 2*time.Minute, 5*time.Second, func() error {
			svc, err := kubeClient.CoreV1().Services(dashboardNamespace).Get(context.Background(), "kubernetes-dashboard", metav1.GetOptions{})
			if err != nil {
				return err
			}
			if len(svc.Spec.Ports) == 0 {
				return fmt.Errorf("dashboard service has no ports yet")
			}
			dashboardService = svc
			return nil
		})

		_ = kubeClient.NetworkingV1().Ingresses(dashboardNamespace).Delete(context.Background(), dashboardIngressName, metav1.DeleteOptions{})

		By("creating an Ingress bound to the Cloudflare tunnel ingress class")
		pathType := networkingv1.PathTypePrefix
		ingress := &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dashboardIngressName,
				Namespace: dashboardNamespace,
			},
			Spec: networkingv1.IngressSpec{
				IngressClassName: ptr.To("cloudflare-tunnel"),
				Rules: []networkingv1.IngressRule{
					{
						Host: dashboardHostname,
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path:     "/",
										PathType: &pathType,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: dashboardService.Name,
												Port: networkingv1.ServiceBackendPort{Number: dashboardService.Spec.Ports[0].Port},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		_, err := kubeClient.NetworkingV1().Ingresses(dashboardNamespace).Create(context.Background(), ingress, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		DeferCleanup(func() {
			deleteCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if err := kubeClient.NetworkingV1().Ingresses(dashboardNamespace).Delete(deleteCtx, dashboardIngressName, metav1.DeleteOptions{}); err != nil {
				GinkgoWriter.Write([]byte(fmt.Sprintf("warning: failed to delete ingress %s/%s: %v\n", dashboardNamespace, dashboardIngressName, err)))
			}
		})

		By("waiting for the ingress status to include the Cloudflare tunnel hostname")
		waitFor("ingress status hostname", 10*time.Minute, 10*time.Second, func() error {
			current, err := kubeClient.NetworkingV1().Ingresses(dashboardNamespace).Get(context.Background(), dashboardIngressName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if len(current.Status.LoadBalancer.Ingress) == 0 {
				return fmt.Errorf("ingress status has no entries yet")
			}
			for _, lb := range current.Status.LoadBalancer.Ingress {
				if strings.TrimSpace(lb.Hostname) != "" {
					return nil
				}
			}
			return fmt.Errorf("ingress status entries have empty hostnames")
		})

		By("waiting for Cloudflare to serve the dashboard over HTTPS")
		client := &http.Client{Timeout: 30 * time.Second}
		dashboardURL := fmt.Sprintf("https://%s/", dashboardHostname)
		waitFor("cloudflare https availability", 15*time.Minute, 20*time.Second, func() error {
			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, dashboardURL, nil)
			if err != nil {
				return err
			}
			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("unexpected status code %d", resp.StatusCode)
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			if !strings.Contains(string(body), "Kubernetes Dashboard") {
				return fmt.Errorf("response body does not contain expected marker")
			}
			return nil
		})

		if path, err := captureDashboardScreenshot(context.Background(), dashboardURL); err != nil {
			GinkgoWriter.Write([]byte(fmt.Sprintf("warning: failed to capture dashboard screenshot: %v\n", err)))
		} else {
			GinkgoWriter.Write([]byte(fmt.Sprintf("dashboard screenshot saved to %s\n", path)))
		}
	})
})

func isNodeReady(node corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func isDeploymentAvailable(deployment appsv1.Deployment) bool {
	return deployment.Status.AvailableReplicas >= 1
}

func waitFor(description string, timeout, interval time.Duration, fn func() error) {
	start := time.Now()
	Eventually(func() error {
		err := fn()
		if err != nil {
			GinkgoWriter.Write([]byte(fmt.Sprintf("[%s pending %s] %v\n", description, time.Since(start).Round(time.Second), err)))
		}
		return err
	}, timeout, interval).Should(Succeed())
	GinkgoWriter.Write([]byte(fmt.Sprintf("[%s] completed in %s\n", description, time.Since(start).Round(time.Second))))
}

func captureDashboardScreenshot(ctx context.Context, url string) (string, error) {
	if repoRoot == "" {
		return "", fmt.Errorf("repository root not resolved")
	}

	browserCtx, cancel := chromedp.NewContext(ctx)
	defer cancel()

	timeoutCtx, cancelTimeout := context.WithTimeout(browserCtx, 2*time.Minute)
	defer cancelTimeout()

	var imageBytes []byte
	tasks := chromedp.Tasks{
		chromedp.Navigate(url),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(2 * time.Second),
		chromedp.FullScreenshot(&imageBytes, 90),
	}

	if err := chromedp.Run(timeoutCtx, tasks...); err != nil {
		return "", err
	}

	artifactsDir := filepath.Join(repoRoot, "test", "e2e", "artifacts")
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		return "", err
	}

	filename := fmt.Sprintf("dashboard-%s.png", time.Now().Format("2006-01-02_15-04-05"))
	path := filepath.Join(artifactsDir, filename)
	if err := os.WriteFile(path, imageBytes, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
