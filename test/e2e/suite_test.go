package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/cucumber/godog/colors"
	"github.com/joho/godotenv"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"
)

var (
	kubeconfigPath     string
	minikubeProfile    string
	kubeClient         *kubernetes.Clientset
	suiteCtx           context.Context
	repoRoot           string
	controllerImage    string
	controllerImageRef imageRef
	baseDomain         string
	logOut             io.Writer = os.Stdout
)

var requiredEnvVars = []string{
	"CLOUDFLARE_API_TOKEN",
	"CLOUDFLARE_ACCOUNT_ID",
	"CLOUDFLARE_TUNNEL_NAME",
	controllerImageEnvKey,
	baseDomainEnvKey,
	e2eKubeconfigEnvKey,
	e2eMinikubeProfileEnvKey,
}

const (
	dotenvPath               = ".env.e2e"
	tokenVerifyURL           = "https://api.cloudflare.com/client/v4/user/tokens/verify"
	controllerImageEnvKey    = "E2E_CONTROLLER_IMAGE"
	baseDomainEnvKey         = "E2E_BASE_DOMAIN"
	e2eKubeconfigEnvKey      = "E2E_KUBECONFIG"
	e2eMinikubeProfileEnvKey = "E2E_MINIKUBE_PROFILE"
	e2eClusterDomain         = "e2e.cluster.internal"
	controllerNamespace      = "cloudflare-tunnel-ingress-controller"
	controllerReleaseName    = "cf-ic-e2e"
)

type imageRef struct {
	repository string
	tag        string
}

func TestE2E(t *testing.T) {
	if err := os.MkdirAll("artifacts", 0o755); err != nil {
		t.Fatalf("create artifacts directory: %v", err)
	}

	opts := godog.Options{
		Format:      "pretty,junit:artifacts/godog-junit.xml",
		Output:      colors.Colored(os.Stdout),
		Paths:       []string{"features"},
		Concurrency: 1,
		Strict:      true,
		TestingT:    t,
	}

	status := godog.TestSuite{
		Name:                 "cloudflare-tunnel-ingress-controller",
		TestSuiteInitializer: InitializeTestSuite,
		ScenarioInitializer:  InitializeScenario,
		Options:              &opts,
	}.Run()

	if status != 0 {
		t.Fatalf("godog suite exited with status %d", status)
	}
}

// InitializeTestSuite boots the shared infrastructure once for all scenarios:
// minikube, the controller image, the Helm release. Scenarios only manage
// their own workloads and ingresses on top of it.
func InitializeTestSuite(ctx *godog.TestSuiteContext) {
	ctx.BeforeSuite(func() {
		if err := bootstrapSuite(); err != nil {
			panic(fmt.Sprintf("bootstrap e2e suite: %v", err))
		}
	})

	ctx.AfterSuite(func() {
		if err := collectControllerCoverage(controllerNamespace, controllerReleaseName); err != nil {
			_, _ = fmt.Fprintf(logOut, "warning: failed to collect coverage: %v\n", err)
		}
	})
}

func bootstrapSuite() error {
	suiteCtx = context.Background()

	if err := loadDotenv(dotenvPath); err != nil {
		return err
	}

	if missing := missingEnvVars(requiredEnvVars); len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	controllerImage = os.Getenv(controllerImageEnvKey)
	var err error
	controllerImageRef, err = parseImageRef(controllerImage)
	if err != nil {
		return fmt.Errorf("parse controller image reference: %w", err)
	}
	baseDomain = os.Getenv(baseDomainEnvKey)

	verifyCtx, cancelVerify := context.WithTimeout(suiteCtx, 30*time.Second)
	defer cancelVerify()
	if err := verifyCloudflareToken(verifyCtx, os.Getenv("CLOUDFLARE_API_TOKEN")); err != nil {
		return err
	}

	for _, binary := range []string{"minikube", "helm", "cloudflared"} {
		if _, err := exec.LookPath(binary); err != nil {
			return fmt.Errorf("%s binary must be installed and on PATH", binary)
		}
	}

	minikubeProfile = os.Getenv(e2eMinikubeProfileEnvKey)
	kubeconfigPath = os.Getenv(e2eKubeconfigEnvKey)

	startCtx, cancelStart := context.WithTimeout(suiteCtx, 20*time.Minute)
	defer cancelStart()
	startCmd := exec.CommandContext(startCtx, "minikube", "start", "-p", minikubeProfile, "--wait=all", fmt.Sprintf("--dns-domain=%s", e2eClusterDomain))
	startCmd.Stdout = logOut
	startCmd.Stderr = logOut
	if err := startCmd.Run(); err != nil {
		return fmt.Errorf("start minikube profile %s: %w", minikubeProfile, err)
	}

	kubeconfigData, err := fetchKubeconfig(suiteCtx, minikubeProfile)
	if err != nil {
		return fmt.Errorf("fetch kubeconfig for profile %s: %w", minikubeProfile, err)
	}
	if err := os.WriteFile(kubeconfigPath, kubeconfigData, 0o600); err != nil {
		return fmt.Errorf("write kubeconfig temp file: %w", err)
	}
	if err := os.Setenv("KUBECONFIG", kubeconfigPath); err != nil {
		return fmt.Errorf("set KUBECONFIG: %w", err)
	}

	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return fmt.Errorf("build rest config: %w", err)
	}
	kubeClient, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("init kube client: %w", err)
	}

	if err := waitFor("nodes ready", 10*time.Minute, 10*time.Second, func() error {
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
	}); err != nil {
		return err
	}

	loadCtx, cancelLoad := context.WithTimeout(suiteCtx, 5*time.Minute)
	defer cancelLoad()
	if err := loadImageIntoMinikube(loadCtx, minikubeProfile, controllerImage); err != nil {
		return err
	}

	values := controllerHelmValues{}
	values.Cloudflare.AccountID = os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	values.Cloudflare.TunnelName = os.Getenv("CLOUDFLARE_TUNNEL_NAME")
	values.Cloudflare.APIToken = os.Getenv("CLOUDFLARE_API_TOKEN")
	values.Image.Repository = controllerImageRef.repository
	values.Image.Tag = controllerImageRef.tag
	values.Image.PullPolicy = "IfNotPresent"
	values.ClusterDomain = e2eClusterDomain

	helmCtx, cancelHelm := context.WithTimeout(suiteCtx, 10*time.Minute)
	defer cancelHelm()
	if err := helmUpgradeInstall(helmCtx, kubeconfigPath, controllerReleaseName, controllerNamespace, values); err != nil {
		return err
	}

	return waitFor("controller deployment ready", 10*time.Minute, 10*time.Second, func() error {
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
}

func missingEnvVars(keys []string) []string {
	var missing []string
	for _, key := range keys {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			missing = append(missing, key)
		}
	}
	return missing
}

func loadDotenv(path string) error {
	resolved, err := resolveDotenvPath(path)
	if err != nil {
		return err
	}
	repoRoot = filepath.Dir(resolved)
	if err := godotenv.Overload(resolved); err != nil {
		return fmt.Errorf("load dotenv %s: %w", resolved, err)
	}
	return nil
}

func resolveDotenvPath(path string) (string, error) {
	if filepath.IsAbs(path) {
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("check dotenv file %s: %w", path, err)
		}
		return path, nil
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	for {
		candidate := filepath.Join(dir, path)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("check dotenv file %s: %w", candidate, err)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("dotenv file %s not found in current or parent directories", path)
}

type tokenVerifyResponse struct {
	Success bool `json:"success"`
	Errors  []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func verifyCloudflareToken(ctx context.Context, token string) error {
	if token == "" {
		return fmt.Errorf("cloudflare api token is empty")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenVerifyURL, nil)
	if err != nil {
		return fmt.Errorf("build token verify request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("perform token verify request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token verify request returned status %d", resp.StatusCode)
	}

	var payload tokenVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return fmt.Errorf("decode token verify response: %w", err)
	}

	if !payload.Success {
		var messages []string
		for _, item := range payload.Errors {
			messages = append(messages, item.Message)
		}
		return fmt.Errorf("cloudflare token verification failed: %s", strings.Join(messages, "; "))
	}

	return nil
}

func fetchKubeconfig(ctx context.Context, profile string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "minikube", "-p", profile, "kubectl", "--", "config", "view", "--raw")
	cmd.Stderr = logOut
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("minikube kubectl config view: %w", err)
	}
	if out.Len() == 0 {
		return nil, fmt.Errorf("minikube kubectl returned empty kubeconfig")
	}
	return out.Bytes(), nil
}

type controllerHelmValues struct {
	Cloudflare struct {
		AccountID  string `yaml:"accountId"`
		TunnelName string `yaml:"tunnelName"`
		APIToken   string `yaml:"apiToken"`
	} `yaml:"cloudflare"`
	Image struct {
		Repository string `yaml:"repository"`
		Tag        string `yaml:"tag"`
		PullPolicy string `yaml:"pullPolicy"`
	} `yaml:"image"`
	ClusterDomain string `yaml:"clusterDomain,omitempty"`
}

func parseImageRef(ref string) (imageRef, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return imageRef{}, fmt.Errorf("image reference is empty")
	}
	lastSlash := strings.LastIndex(ref, "/")
	lastColon := strings.LastIndex(ref, ":")
	if lastColon > lastSlash {
		return imageRef{repository: ref[:lastColon], tag: ref[lastColon+1:]}, nil
	}
	return imageRef{repository: ref, tag: "latest"}, nil
}

func loadImageIntoMinikube(ctx context.Context, profile string, image string) error {
	cmd := exec.CommandContext(ctx, "minikube", "-p", profile, "image", "load", image)
	cmd.Stdout = logOut
	cmd.Stderr = logOut
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("minikube image load %s: %w", image, err)
	}
	return nil
}

func helmUpgradeInstall(ctx context.Context, kubeconfigPath string, releaseName string, namespace string, values controllerHelmValues) error {
	if repoRoot == "" {
		return fmt.Errorf("repository root not resolved")
	}
	chartPath := filepath.Join(repoRoot, "helm", "cloudflare-tunnel-ingress-controller")
	valuesPath, err := writeHelmValuesFile(values)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(valuesPath) }()

	_, _ = fmt.Fprintf(logOut, "helm image override: repository=%s tag=%s pullPolicy=%s\n", values.Image.Repository, values.Image.Tag, values.Image.PullPolicy)
	_, _ = fmt.Fprintf(logOut, "helm cloudflare values length: accountId=%d tunnelName=%d apiToken=%d\n",
		len(values.Cloudflare.AccountID), len(values.Cloudflare.TunnelName), len(values.Cloudflare.APIToken))

	helmArgs := []string{
		"upgrade", "--install", releaseName, chartPath,
		"--namespace", namespace,
		"--create-namespace",
		"--wait",
		"--timeout", "10m0s",
		"-f", valuesPath,
	}

	if strings.TrimSpace(values.Cloudflare.AccountID) != "" {
		helmArgs = append(helmArgs, "--set-string", fmt.Sprintf("cloudflare.accountId=%s", values.Cloudflare.AccountID))
	}
	if strings.TrimSpace(values.Cloudflare.TunnelName) != "" {
		helmArgs = append(helmArgs, "--set-string", fmt.Sprintf("cloudflare.tunnelName=%s", values.Cloudflare.TunnelName))
	}
	if strings.TrimSpace(values.Cloudflare.APIToken) != "" {
		helmArgs = append(helmArgs, "--set-string", fmt.Sprintf("cloudflare.apiToken=%s", values.Cloudflare.APIToken))
	}
	if strings.TrimSpace(values.Image.Repository) != "" {
		helmArgs = append(helmArgs, "--set-string", fmt.Sprintf("image.repository=%s", values.Image.Repository))
	}
	if strings.TrimSpace(values.Image.Tag) != "" {
		helmArgs = append(helmArgs, "--set-string", fmt.Sprintf("image.tag=%s", values.Image.Tag))
	}
	if strings.TrimSpace(values.Image.PullPolicy) != "" {
		helmArgs = append(helmArgs, "--set-string", fmt.Sprintf("image.pullPolicy=%s", values.Image.PullPolicy))
	}
	if strings.TrimSpace(values.ClusterDomain) != "" {
		helmArgs = append(helmArgs, "--set-string", fmt.Sprintf("clusterDomain=%s", values.ClusterDomain))
	}

	cmd := exec.CommandContext(ctx, "helm", helmArgs...)
	cmd.Stdout = logOut
	cmd.Stderr = logOut
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("helm upgrade --install %s: %w", releaseName, err)
	}
	return nil
}

func writeHelmValuesFile(values controllerHelmValues) (string, error) {
	data, err := yaml.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("marshal helm values: %w", err)
	}
	file, err := os.CreateTemp("", "cf-ic-helm-values-*.yaml")
	if err != nil {
		return "", fmt.Errorf("create helm values temp file: %w", err)
	}
	if _, err = file.Write(data); err != nil {
		_ = os.Remove(file.Name())
		return "", fmt.Errorf("write helm values file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(file.Name())
		return "", fmt.Errorf("close helm values file: %w", err)
	}
	return file.Name(), nil
}

func enableMinikubeAddon(ctx context.Context, profile string, addon string) error {
	cmd := exec.CommandContext(ctx, "minikube", "-p", profile, "addons", "enable", addon)
	cmd.Stdout = logOut
	cmd.Stderr = logOut
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("enable minikube addon %s: %w", addon, err)
	}
	return nil
}

func buildTestHostname(prefix string, baseDomain string) (string, error) {
	trimmed := strings.TrimSpace(baseDomain)
	trimmed = strings.TrimPrefix(trimmed, "https://")
	trimmed = strings.TrimPrefix(trimmed, "http://")
	trimmed = strings.TrimPrefix(trimmed, "//")
	trimmed = strings.Trim(trimmed, ".")
	if trimmed == "" {
		return "", fmt.Errorf("base domain is empty")
	}
	if strings.Contains(trimmed, "/") {
		return "", fmt.Errorf("base domain %s must not contain path", trimmed)
	}
	label := fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	return fmt.Sprintf("%s.%s", label, trimmed), nil
}

func collectControllerCoverage(namespace string, releaseName string) error {
	if repoRoot == "" {
		return fmt.Errorf("repository root not resolved")
	}
	if kubeClient == nil {
		return fmt.Errorf("kube client not initialised")
	}

	pods, err := kubeClient.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/instance=%s", releaseName),
	})
	if err != nil {
		return fmt.Errorf("list controller pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no controller pods found")
	}
	podName := pods.Items[0].Name
	_, _ = fmt.Fprintf(logOut, "collecting coverage from pod %s/%s\n", namespace, podName)

	signalCtx, cancelSignal := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelSignal()
	signalCmd := exec.CommandContext(signalCtx, "kubectl", "exec", podName, "-n", namespace, "--", "kill", "-USR1", "1")
	signalCmd.Stdout = logOut
	signalCmd.Stderr = logOut
	if err := signalCmd.Run(); err != nil {
		return fmt.Errorf("send SIGUSR1 to controller: %w", err)
	}

	time.Sleep(2 * time.Second)

	coverageDir := filepath.Join(repoRoot, "test", "e2e", "artifacts", "coverage")
	if err := os.MkdirAll(coverageDir, 0o755); err != nil {
		return fmt.Errorf("create coverage directory: %w", err)
	}

	extractCtx, cancelExtract := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelExtract()
	tarCmd := exec.CommandContext(extractCtx, "kubectl", "exec", podName, "-n", namespace, "--", "tar", "-cf", "-", "-C", "/tmp/coverage", ".")
	tarData, err := tarCmd.Output()
	if err != nil {
		return fmt.Errorf("extract coverage data from pod: %w", err)
	}

	untarCmd := exec.Command("tar", "-xf", "-", "-C", coverageDir)
	untarCmd.Stdin = bytes.NewReader(tarData)
	untarCmd.Stdout = logOut
	untarCmd.Stderr = logOut
	if err := untarCmd.Run(); err != nil {
		return fmt.Errorf("untar coverage data: %w", err)
	}

	coverOut := filepath.Join(repoRoot, "test", "e2e", "artifacts", "e2e-cover.out")
	covdataCmd := exec.Command("go", "tool", "covdata", "textfmt", "-i="+coverageDir, "-o="+coverOut)
	covdataCmd.Stdout = logOut
	covdataCmd.Stderr = logOut
	if err := covdataCmd.Run(); err != nil {
		return fmt.Errorf("convert coverage data to text format: %w", err)
	}

	_, _ = fmt.Fprintf(logOut, "e2e coverage data saved to %s\n", coverOut)
	return nil
}
