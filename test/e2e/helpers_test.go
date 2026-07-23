package e2e

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

var edgeHTTPClientOnce = sync.OnceValue(newEdgeHTTPClient)

func edgeHTTPClient() *http.Client {
	return edgeHTTPClientOnce()
}

// newEdgeHTTPClient resolves via 1.1.1.1 directly instead of the local stub
// resolver. Records are created by the controller while the test runs, and
// systemd-resolved would otherwise cache the initial NXDOMAIN for up to the
// zone's negative TTL, keeping probes failing long after the record exists.
func newEdgeHTTPClient() *http.Client {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			dialer := net.Dialer{Timeout: 5 * time.Second}
			return dialer.DialContext(ctx, "udp", "1.1.1.1:53")
		},
	}
	dialer := &net.Dialer{
		Timeout:  10 * time.Second,
		Resolver: resolver,
	}
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext: dialer.DialContext,
		},
	}
}

func createHTTPEcho(namespace string, name string, text string) error {
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
							Name:  "http-echo",
							Image: "hashicorp/http-echo:1.0",
							Args:  []string{fmt.Sprintf("-text=%s", text)},
							Ports: []corev1.ContainerPort{{ContainerPort: 5678}},
						},
					},
				},
			},
		},
	}
	if _, err := kubeClient.AppsV1().Deployments(namespace).Create(context.Background(), deployment, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create %s deployment: %w", name, err)
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
					Port:       80,
					TargetPort: intstr.FromInt32(5678),
				},
			},
		},
	}
	if _, err := kubeClient.CoreV1().Services(namespace).Create(context.Background(), service, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create %s service: %w", name, err)
	}
	return nil
}

func expectHTTPBody(client *http.Client, url string, marker string) error {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
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
	if !strings.Contains(string(body), marker) {
		return fmt.Errorf("response body %q does not contain %q", strings.TrimSpace(string(body)), marker)
	}
	return nil
}

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

func waitFor(description string, timeout, interval time.Duration, fn func() error) error {
	start := time.Now()
	deadline := start.Add(timeout)
	for {
		err := fn()
		if err == nil {
			_, _ = fmt.Fprintf(logOut, "[%s] completed in %s\n", description, time.Since(start).Round(time.Second))
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s: timed out after %s: %w", description, timeout, err)
		}
		_, _ = fmt.Fprintf(logOut, "[%s pending %s] %v\n", description, time.Since(start).Round(time.Second), err)
		time.Sleep(interval)
	}
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
