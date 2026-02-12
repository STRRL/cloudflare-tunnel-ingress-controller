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
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
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

		err = controller.CreateOrUpdateControlledCloudflared(ctx, kubeClient, mockTunnelClient, ns, protocol, []string{}, nil, "")
		Expect(err).NotTo(HaveOccurred())

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

		err = controller.CreateOrUpdateControlledCloudflared(ctx, kubeClient, mockTunnelClient, ns, protocol, []string{}, nil, "")
		Expect(err).NotTo(HaveOccurred())

		Expect(os.Setenv("CLOUDFLARED_REPLICA_COUNT", "3")).To(Succeed())
		Expect(os.Setenv("CLOUDFLARED_IMAGE", "cloudflare/cloudflared:2022.3.0")).To(Succeed())

		err = controller.CreateOrUpdateControlledCloudflared(ctx, kubeClient, mockTunnelClient, ns, protocol, []string{}, nil, "")
		Expect(err).NotTo(HaveOccurred())

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

		err = controller.CreateOrUpdateControlledCloudflared(ctx, kubeClient, mockTunnelClient, ns, protocol, extraArgs, nil, "")
		Expect(err).NotTo(HaveOccurred())

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

	It("should apply deployment config to cloudflared deployment", func() {
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
		deploymentConfig := &controller.CloudflaredDeploymentConfig{
			Resources: &v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("100m"),
					v1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Limits: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("200m"),
					v1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
			SecurityContext: &v1.SecurityContext{
				ReadOnlyRootFilesystem: ptr.To(true),
				RunAsNonRoot:           ptr.To(true),
			},
			PodSecurityContext: &v1.PodSecurityContext{
				RunAsNonRoot: ptr.To(true),
			},
			PodLabels: map[string]string{
				"team": "platform",
			},
			PodAnnotations: map[string]string{
				"prometheus.io/scrape": "true",
			},
			NodeSelector: map[string]string{
				"kubernetes.io/os": "linux",
			},
			PriorityClassName: "high-priority",
		}

		err = controller.CreateOrUpdateControlledCloudflared(ctx, kubeClient, mockTunnelClient, ns, protocol, []string{}, deploymentConfig, "test-hash")
		Expect(err).NotTo(HaveOccurred())

		deployment := &appsv1.Deployment{}
		err = kubeClient.Get(ctx, types.NamespacedName{
			Namespace: ns,
			Name:      "controlled-cloudflared-connector",
		}, deployment)
		Expect(err).NotTo(HaveOccurred())

		container := deployment.Spec.Template.Spec.Containers[0]
		Expect(container.Resources.Requests.Cpu().String()).To(Equal("100m"))
		Expect(container.Resources.Requests.Memory().String()).To(Equal("128Mi"))
		Expect(container.Resources.Limits.Cpu().String()).To(Equal("200m"))
		Expect(container.Resources.Limits.Memory().String()).To(Equal("256Mi"))

		Expect(container.SecurityContext).NotTo(BeNil())
		Expect(*container.SecurityContext.ReadOnlyRootFilesystem).To(BeTrue())
		Expect(*container.SecurityContext.RunAsNonRoot).To(BeTrue())

		Expect(deployment.Spec.Template.Spec.SecurityContext).NotTo(BeNil())
		Expect(*deployment.Spec.Template.Spec.SecurityContext.RunAsNonRoot).To(BeTrue())

		Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue("team", "platform"))
		Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue("app", "controlled-cloudflared-connector"))
		Expect(deployment.Spec.Template.Annotations).To(HaveKeyWithValue("prometheus.io/scrape", "true"))

		Expect(deployment.Spec.Template.Spec.NodeSelector).To(HaveKeyWithValue("kubernetes.io/os", "linux"))
		Expect(deployment.Spec.Template.Spec.PriorityClassName).To(Equal("high-priority"))

		Expect(deployment.Annotations).To(HaveKeyWithValue("strrl.dev/cloudflared-config-hash", "test-hash"))
	})

	It("should update deployment when config hash changes", func() {
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

		// Create with initial config
		config1 := &controller.CloudflaredDeploymentConfig{
			PriorityClassName: "low-priority",
		}
		err = controller.CreateOrUpdateControlledCloudflared(ctx, kubeClient, mockTunnelClient, ns, protocol, []string{}, config1, "hash-v1")
		Expect(err).NotTo(HaveOccurred())

		// Update with new config (different hash)
		config2 := &controller.CloudflaredDeploymentConfig{
			PriorityClassName: "high-priority",
		}
		err = controller.CreateOrUpdateControlledCloudflared(ctx, kubeClient, mockTunnelClient, ns, protocol, []string{}, config2, "hash-v2")
		Expect(err).NotTo(HaveOccurred())

		deployment := &appsv1.Deployment{}
		err = kubeClient.Get(ctx, types.NamespacedName{
			Namespace: ns,
			Name:      "controlled-cloudflared-connector",
		}, deployment)
		Expect(err).NotTo(HaveOccurred())

		Expect(deployment.Spec.Template.Spec.PriorityClassName).To(Equal("high-priority"))
		Expect(deployment.Annotations).To(HaveKeyWithValue("strrl.dev/cloudflared-config-hash", "hash-v2"))
	})
})
