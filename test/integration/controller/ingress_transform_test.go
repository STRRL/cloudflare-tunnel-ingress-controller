package controller

import (
	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/controller"
	"github.com/STRRL/cloudflare-tunnel-ingress-controller/test/fixtures"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var IntegrationTestNamespace = "cf-tunnel-ingress-controller-test"

var pathTypePrefix = networkingv1.PathTypePrefix

var _ = Describe("transform ingress to exposure", func() {
	It("should resolve ingress with PathType Prefix", func() {
		// prepare
		By("preparing namespace")
		namespaceFixtures := fixtures.NewKubernetesNamespaceFixtures(IntegrationTestNamespace, kubeClient)
		ns, err := namespaceFixtures.Start(ctx)
		Expect(err).ShouldNot(HaveOccurred())

		defer func() {
			By("cleaning up namespace")
			err := namespaceFixtures.Stop(ctx)
			Expect(err).ShouldNot(HaveOccurred())
		}()

		By("preparing service")
		service := v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    ns,
				GenerateName: "test-service-",
			},
			Spec: v1.ServiceSpec{
				ClusterIP: "10.0.0.23",
				Ports: []v1.ServicePort{
					{
						Name:     "http",
						Protocol: v1.ProtocolTCP,
						Port:     2333,
						TargetPort: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: 8080,
						},
					},
				},
			},
		}
		err = kubeClient.Create(ctx, &service)
		Expect(err).ShouldNot(HaveOccurred())

		By("preparing ingress")
		ingress := networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    ns,
				GenerateName: "test-ingress-",
			},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{
					{
						Host: "test.example.com",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path:     "/",
										PathType: &pathTypePrefix,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: service.Name,
												Port: networkingv1.ServiceBackendPort{
													Number: 2333,
												},
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
		err = kubeClient.Create(ctx, &ingress)
		Expect(err).ShouldNot(HaveOccurred())

		exposure, err := controller.FromIngressToExposure(ctx, kubeClient, ingress)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(exposure).ShouldNot(BeNil())
		Expect(exposure).Should(HaveLen(1))
		Expect(exposure[0].Hostname).Should(Equal("test.example.com"))
		Expect(exposure[0].ServiceTarget).Should(Equal("http://10.0.0.23:2333"))
		Expect(exposure[0].PathPrefix).Should(Equal("/"))
		Expect(exposure[0].IsDeleted).Should(BeFalse())
	})
})
