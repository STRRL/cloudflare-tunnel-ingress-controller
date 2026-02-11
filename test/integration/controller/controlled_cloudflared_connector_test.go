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
		Expect(os.Setenv("CLOUDFLARED_REPLICA_COUNT", "2")).To(Succeed())
		Expect(os.Setenv("CLOUDFLARED_IMAGE", "cloudflare/cloudflared:latest")).To(Succeed())
		Expect(os.Setenv("CLOUDFLARED_IMAGE_PULL_POLICY", "IfNotPresent")).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.Unsetenv("CLOUDFLARED_REPLICA_COUNT")).To(Succeed())
		Expect(os.Unsetenv("CLOUDFLARED_IMAGE")).To(Succeed())
		Expect(os.Unsetenv("CLOUDFLARED_IMAGE_PULL_POLICY")).To(Succeed())
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

		protocol := "quic"

		// Act
		err = controller.CreateOrUpdateControlledCloudflared(ctx, kubeClient, mockTunnelClient, ns, protocol, []string{})
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

		protocol := "quic"

		// Create initial deployment
		err = controller.CreateOrUpdateControlledCloudflared(ctx, kubeClient, mockTunnelClient, ns, protocol, []string{})
		Expect(err).NotTo(HaveOccurred())

		Expect(os.Setenv("CLOUDFLARED_REPLICA_COUNT", "3")).To(Succeed())
		Expect(os.Setenv("CLOUDFLARED_IMAGE", "cloudflare/cloudflared:2022.3.0")).To(Succeed())

		// Act
		err = controller.CreateOrUpdateControlledCloudflared(ctx, kubeClient, mockTunnelClient, ns, protocol, []string{})
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

	It("should include extra args in cloudflared command", func() {
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

		protocol := "quic"
		extraArgs := []string{"--post-quantum", "--edge-ip-version", "4"}

		// Act
		err = controller.CreateOrUpdateControlledCloudflared(ctx, kubeClient, mockTunnelClient, ns, protocol, extraArgs)
		Expect(err).NotTo(HaveOccurred())

		// Assert
		deployment := &appsv1.Deployment{}
		err = kubeClient.Get(ctx, types.NamespacedName{
			Namespace: ns,
			Name:      "controlled-cloudflared-connector",
		}, deployment)
		Expect(err).NotTo(HaveOccurred())

		command := deployment.Spec.Template.Spec.Containers[0].Command
		Expect(command).To(ContainElement("--post-quantum"))
		Expect(command).To(ContainElement("--edge-ip-version"))
		Expect(command).To(ContainElement("4"))
	})
})
