package e2e

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

var _ = Describe("Happy Path", func() {
	const (
		controllerNamespace   = "cloudflare-tunnel-ingress-controller"
		controllerReleaseName = "cf-ic-e2e"
		dashboardNamespace    = "kubernetes-dashboard"
		dashboardIngressName  = "dashboard-via-cloudflare"
		redisNamespace        = "default"
		redisName             = "e2e-redis"
		redisPort             = int32(6379)
		tcpIngressName        = "redis-via-cloudflare-tcp"
		redisLocalAddr        = "127.0.0.1:16379"
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
		values.ClusterDomain = e2eClusterDomain

		helmCtx, cancelHelm := context.WithTimeout(suiteCtx, 10*time.Minute)
		Expect(helmUpgradeInstall(helmCtx, kubeconfigPath, controllerReleaseName, controllerNamespace, values)).To(Succeed())
		cancelHelm()

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
			defer func() { _ = resp.Body.Close() }()
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
			_, _ = fmt.Fprintf(GinkgoWriter, "warning: failed to capture dashboard screenshot: %v\n", err)
		} else {
			_, _ = fmt.Fprintf(GinkgoWriter, "dashboard screenshot saved to %s\n", path)
		}

		By("deploying a redis instance for the tcp exposure")
		Expect(createRedis(redisNamespace, redisName)).To(Succeed())
		waitFor("redis deployment ready", 5*time.Minute, 5*time.Second, func() error {
			deployment, err := kubeClient.AppsV1().Deployments(redisNamespace).Get(context.Background(), redisName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if !isDeploymentAvailable(*deployment) {
				return fmt.Errorf("redis deployment has %d available replicas", deployment.Status.AvailableReplicas)
			}
			return nil
		})

		By("creating an Ingress exposing redis with backend-protocol tcp")
		_ = kubeClient.NetworkingV1().Ingresses(redisNamespace).Delete(context.Background(), tcpIngressName, metav1.DeleteOptions{})
		tcpIngress := &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tcpIngressName,
				Namespace: redisNamespace,
				Annotations: map[string]string{
					"cloudflare-tunnel-ingress-controller.strrl.dev/backend-protocol": "tcp",
				},
			},
			Spec: networkingv1.IngressSpec{
				IngressClassName: ptr.To("cloudflare-tunnel"),
				Rules: []networkingv1.IngressRule{
					{
						Host: tcpHostname,
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path:     "/",
										PathType: &pathType,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: redisName,
												Port: networkingv1.ServiceBackendPort{Number: redisPort},
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
		_, err = kubeClient.NetworkingV1().Ingresses(redisNamespace).Create(context.Background(), tcpIngress, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("waiting for the tcp ingress status to include the Cloudflare tunnel hostname")
		waitFor("tcp ingress status hostname", 10*time.Minute, 10*time.Second, func() error {
			current, err := kubeClient.NetworkingV1().Ingresses(redisNamespace).Get(context.Background(), tcpIngressName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if len(current.Status.LoadBalancer.Ingress) == 0 {
				return fmt.Errorf("tcp ingress status has no entries yet")
			}
			return nil
		})

		By("reaching redis through the tunnel via cloudflared access tcp")
		accessCtx, cancelAccess := context.WithCancel(suiteCtx)
		accessCmd := exec.CommandContext(accessCtx, "cloudflared", "access", "tcp", "--hostname", tcpHostname, "--url", redisLocalAddr)
		accessCmd.Stdout = GinkgoWriter
		accessCmd.Stderr = GinkgoWriter
		Expect(accessCmd.Start()).To(Succeed())
		defer func() {
			cancelAccess()
			_ = accessCmd.Wait()
		}()

		waitFor("redis PING via tunnel", 15*time.Minute, 20*time.Second, func() error {
			return redisPing(redisLocalAddr)
		})

		By("collecting coverage data from the controller pod")
		if err := collectControllerCoverage(controllerNamespace, controllerReleaseName); err != nil {
			_, _ = fmt.Fprintf(GinkgoWriter, "warning: failed to collect coverage: %v\n", err)
		}
	})
})

func createRedis(namespace string, name string) error {
	labels := map[string]string{"app": name}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "redis",
							Image: "redis:7-alpine",
							Ports: []corev1.ContainerPort{{ContainerPort: 6379}},
						},
					},
				},
			},
		},
	}
	if _, err := kubeClient.AppsV1().Deployments(namespace).Create(context.Background(), deployment, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create redis deployment: %w", err)
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Port:       6379,
					TargetPort: intstr.FromInt32(6379),
				},
			},
		},
	}
	if _, err := kubeClient.CoreV1().Services(namespace).Create(context.Background(), service, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create redis service: %w", err)
	}
	return nil
}

// redisPing dials the local forwarder opened by cloudflared access tcp and
// performs a real PING round-trip through the Cloudflare edge and the tunnel.
func redisPing(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	if err := conn.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return err
	}
	if _, err := conn.Write([]byte("PING\r\n")); err != nil {
		return err
	}
	buffer := make([]byte, 64)
	n, err := conn.Read(buffer)
	if err != nil {
		return err
	}
	response := strings.TrimSpace(string(buffer[:n]))
	if response != "+PONG" {
		return fmt.Errorf("unexpected redis response %q", response)
	}
	return nil
}

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
			_, _ = fmt.Fprintf(GinkgoWriter, "[%s pending %s] %v\n", description, time.Since(start).Round(time.Second), err)
		}
		return err
	}, timeout, interval).Should(Succeed())
	_, _ = fmt.Fprintf(GinkgoWriter, "[%s] completed in %s\n", description, time.Since(start).Round(time.Second))
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
