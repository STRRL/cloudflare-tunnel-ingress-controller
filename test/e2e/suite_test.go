package e2e

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/stdr"
	"github.com/joho/godotenv"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	cfg        *rest.Config
	kubeClient client.Client
	ctx        context.Context
	cancel     context.CancelFunc
	e2eConfig  *E2EConfig
)

type E2EConfig struct {
	CloudflareAPIToken         string
	CloudflareAccountID        string
	CloudflareTunnelName       string
	CloudflareTestDomainSuffix string
	TestsEnabled               bool
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cloudflare Tunnel Ingress Controller E2E Suite")
}

var _ = BeforeSuite(func() {
	rootLogger := stdr.NewWithOptions(log.New(os.Stderr, "", log.LstdFlags), stdr.Options{LogCaller: stdr.All})
	logf.SetLogger(rootLogger)
	ctx, cancel = context.WithCancel(context.TODO())

	By("loading E2E configuration")
	var err error
	e2eConfig, err = LoadE2EConfig()
	Expect(err).NotTo(HaveOccurred())

	if !e2eConfig.TestsEnabled {
		Skip("E2E tests are disabled (E2E_TESTS_ENABLED=false)")
	}

	By("connecting to Kubernetes cluster")
	cfg, err = config.GetConfig()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	scheme := runtime.NewScheme()
	err = clientgoscheme.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	kubeClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(kubeClient).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	cancel()
})

func LoadE2EConfig() (*E2EConfig, error) {
	// Try to load .env file from current directory
	envPath := filepath.Join(".", ".env")
	if _, err := os.Stat(envPath); err == nil {
		if err := godotenv.Load(envPath); err != nil {
			return nil, fmt.Errorf("failed to load .env file: %w", err)
		}
	}

	config := &E2EConfig{
		CloudflareAPIToken:         os.Getenv("CLOUDFLARE_API_TOKEN"),
		CloudflareAccountID:        os.Getenv("CLOUDFLARE_ACCOUNT_ID"),
		CloudflareTunnelName:       os.Getenv("CLOUDFLARE_TUNNEL_NAME"),
		CloudflareTestDomainSuffix: os.Getenv("CLOUDFLARE_TEST_DOMAIN_SUFFIX"),
		TestsEnabled:               os.Getenv("E2E_TESTS_ENABLED") == "true",
	}

	// Validate required configuration
	if config.TestsEnabled {
		if config.CloudflareAPIToken == "" {
			return nil, fmt.Errorf("CLOUDFLARE_API_TOKEN is required for E2E tests")
		}
		if config.CloudflareAccountID == "" {
			return nil, fmt.Errorf("CLOUDFLARE_ACCOUNT_ID is required for E2E tests")
		}
		if config.CloudflareTunnelName == "" {
			return nil, fmt.Errorf("CLOUDFLARE_TUNNEL_NAME is required for E2E tests")
		}
		if config.CloudflareTestDomainSuffix == "" {
			return nil, fmt.Errorf("CLOUDFLARE_TEST_DOMAIN_SUFFIX is required for E2E tests")
		}
	}

	return config, nil
}

func GenerateTestHostname() string {
	timestamp := time.Now().Format("20060102-150405")
	random := generateRandomString(6)
	return fmt.Sprintf("e2e-%s-%s.%s", timestamp, random, e2eConfig.CloudflareTestDomainSuffix)
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	rand.Read(b)
	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return string(b)
}
