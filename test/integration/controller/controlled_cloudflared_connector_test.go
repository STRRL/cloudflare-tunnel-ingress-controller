package controller

import (
	"context"
	"os"

	cloudflarecontroller "github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/cloudflare-controller"
	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/controller"
	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/STRRL/cloudflare-tunnel-ingress-controller/test/fixtures"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ cloudflarecontroller.TunnelClientInterface = &MockTunnelClient{}

type MockTunnelClient struct {
	FetchTunnelTokenFunc func(ctx context.Context) (string, error)
}

func (m *MockTunnelClient) PutExposures(ctx context.Context, exposures []exposure.Exposure) error {
	return nil
}

func (m *MockTunnelClient) TunnelDomain() string {
	return "mock.tunnel.com"
}

func (m *MockTunnelClient) FetchTunnelToken(ctx context.Context) (string, error) {
	return m.FetchTunnelTokenFunc(ctx)
}

var _ = Describe("CreateOrUpdateControlledCloudflared", func() {
	const testNamespace = "cloudflared-test"

	BeforeEach(func() {
		// Set required environment variables
		os.Setenv("CLOUDFLARED_REPLICA_COUNT", "2")
		os.Setenv("CLOUDFLARED_IMAGE", "cloudflare/cloudflared:latest")
		os.Setenv("CLOUDFLARED_IMAGE_PULL_POLICY", "IfNotPresent")
	})

	AfterEach(func() {
		// Clean up environment variables
		os.Unsetenv("CLOUDFLARED_REPLICA_COUNT")
		os.Unsetenv("CLOUDFLARED_IMAGE")
		os.Unsetenv("CLOUDFLARED_IMAGE_PULL_POLICY")
	})

	It("should create a new cloudflared deployment", func() {
		// Prepare
		namespaceFixtures := fixtures.NewKubernetesNamespaceFixtures(testNamespace, kubeClient)
		ns, err := namespaceFixtures.Start(ctx)
		Expect(err).NotTo(HaveOccurred())

		defer func() {
			err := namespaceFixtures.Stop(ctx)
			Expect(err).NotTo(HaveOccurred())
		}()

		mockTunnelClient := &MockTunnelClient{
			FetchTunnelTokenFunc: func(ctx context.Context) (string, error) {
				return "mock-token", nil
			},
		}

		// Act
		err = controller.CreateOrUpdateControlledCloudflared(ctx, kubeClient, mockTunnelClient, ns)
		Expect(err).NotTo(HaveOccurred())

		// Assert
		deployment := &appsv1.Deployment{}
		err = kubeClient.Get(ctx, types.NamespacedName{
			Namespace: ns,
			Name:      "controlled-cloudflared-connector",
		}, deployment)
		Expect(err).NotTo(HaveOccurred())

		Expect(*deployment.Spec.Replicas).To(Equal(int32(2)))
		Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("cloudflare/cloudflared:latest"))
		Expect(deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(v1.PullPolicy("IfNotPresent")))
	})

	It("should update an existing cloudflared deployment", func() {
		// Prepare
		namespaceFixtures := fixtures.NewKubernetesNamespaceFixtures(testNamespace, kubeClient)
		ns, err := namespaceFixtures.Start(ctx)
		Expect(err).NotTo(HaveOccurred())

		defer func() {
			err := namespaceFixtures.Stop(ctx)
			Expect(err).NotTo(HaveOccurred())
		}()

		mockTunnelClient := &MockTunnelClient{
			FetchTunnelTokenFunc: func(ctx context.Context) (string, error) {
				return "mock-token", nil
			},
		}

		// Create initial deployment
		err = controller.CreateOrUpdateControlledCloudflared(ctx, kubeClient, mockTunnelClient, ns)
		Expect(err).NotTo(HaveOccurred())

		// Change environment variables
		os.Setenv("CLOUDFLARED_REPLICA_COUNT", "3")
		os.Setenv("CLOUDFLARED_IMAGE", "cloudflare/cloudflared:2022.3.0")

		// Act
		err = controller.CreateOrUpdateControlledCloudflared(ctx, kubeClient, mockTunnelClient, ns)
		Expect(err).NotTo(HaveOccurred())

		// Assert
		deployment := &appsv1.Deployment{}
		err = kubeClient.Get(ctx, types.NamespacedName{
			Namespace: ns,
			Name:      "controlled-cloudflared-connector",
		}, deployment)
		Expect(err).NotTo(HaveOccurred())

		Expect(*deployment.Spec.Replicas).To(Equal(int32(3)))
		Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("cloudflare/cloudflared:2022.3.0"))
	})
})
