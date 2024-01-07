package controller

import (
	"log"
	"os"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/controller"
	"github.com/STRRL/cloudflare-tunnel-ingress-controller/test/fixtures"
	"github.com/go-logr/stdr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const IntegrationTestNamespace = "cf-tunnel-ingress-controller-test"

var pathTypePrefix = networkingv1.PathTypePrefix
var pathTypeExact = networkingv1.PathTypeExact
var pathTypeImplementationSpecific = networkingv1.PathTypeImplementationSpecific

var _ = Describe("transform ingress to exposure", func() {
	logger := stdr.NewWithOptions(log.New(os.Stderr, "", log.LstdFlags), stdr.Options{LogCaller: stdr.All})

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

		By("transforming ingress to exposure")
		exposure, err := controller.FromIngressToExposure(ctx, logger, kubeClient, ingress)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(exposure).ShouldNot(BeNil())
		Expect(exposure).Should(HaveLen(1))
		Expect(exposure[0].Hostname).Should(Equal("test.example.com"))
		Expect(exposure[0].ServiceTarget).Should(Equal("http://10.0.0.23:2333"))
		Expect(exposure[0].PathPrefix).Should(Equal("/"))
		Expect(exposure[0].IsDeleted).Should(BeFalse())
	})

	It("should fail fast with PathType Exact", func() {
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
				ClusterIP: "10.0.0.24",
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
										PathType: &pathTypeExact,
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

		By("transforming ingress to exposure")
		exposure, err := controller.FromIngressToExposure(ctx, logger, kubeClient, ingress)
		Expect(err).Should(HaveOccurred())
		Expect(exposure).Should(BeNil())

		err = kubeClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: service.Name}, &service)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(service.Status.LoadBalancer.Ingress).Should(BeEmpty())
	})

	It("should fail fast with PathType ImplementationSpecific", func() {
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
				ClusterIP: "10.0.0.25",
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
										PathType: &pathTypeImplementationSpecific,
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

		By("transforming ingress to exposure")
		exposure, err := controller.FromIngressToExposure(ctx, logger, kubeClient, ingress)
		Expect(err).Should(HaveOccurred())
		Expect(exposure).Should(BeNil())

		err = kubeClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: service.Name}, &service)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(service.Status.LoadBalancer.Ingress).Should(BeEmpty())
	})

	It("should resolve ingress with port name", func() {
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
				ClusterIP: "10.0.0.26",
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
													Name: "http",
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

		By("transforming ingress to exposure")
		exposure, err := controller.FromIngressToExposure(ctx, logger, kubeClient, ingress)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(exposure).ShouldNot(BeNil())
		Expect(exposure).Should(HaveLen(1))
		Expect(exposure[0].Hostname).Should(Equal("test.example.com"))
		Expect(exposure[0].ServiceTarget).Should(Equal("http://10.0.0.26:2333"))
		Expect(exposure[0].PathPrefix).Should(Equal("/"))
		Expect(exposure[0].IsDeleted).Should(BeFalse())
	})

	It("fail fast if no port found by port name", func() {
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
				ClusterIP: "10.0.0.254",
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
													Name: "whatever-name",
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

		By("transforming ingress to exposure")
		exposure, err := controller.FromIngressToExposure(ctx, logger, kubeClient, ingress)
		Expect(err).Should(HaveOccurred())
		Expect(exposure).Should(BeNil())

		err = kubeClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: service.Name}, &service)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(service.Status.LoadBalancer.Ingress).Should(BeEmpty())
	})

	It("should fail fast with headless service", func() {
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
				ClusterIP: "None",
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
										PathType: &pathTypeExact,
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

		By("transforming ingress to exposure")
		exposure, err := controller.FromIngressToExposure(ctx, logger, kubeClient, ingress)
		Expect(err).Should(HaveOccurred())
		Expect(exposure).Should(BeNil())

		err = kubeClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: service.Name}, &service)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(service.Status.LoadBalancer.Ingress).Should(BeEmpty())
	})

	It("should resolve https", func() {
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
				ClusterIP: "10.0.0.27",
				Ports: []v1.ServicePort{
					{
						Name:     "https",
						Protocol: v1.ProtocolTCP,
						Port:     2333,
						TargetPort: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: 443,
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
				Annotations: map[string]string{
					"cloudflare-tunnel-ingress-controller.strrl.dev/backend-protocol": "https",
				},
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
													Name: "https",
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

		By("transforming ingress to exposure")
		exposure, err := controller.FromIngressToExposure(ctx, logger, kubeClient, ingress)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(exposure).ShouldNot(BeNil())
		Expect(exposure).Should(HaveLen(1))
		Expect(exposure[0].Hostname).Should(Equal("test.example.com"))
		Expect(exposure[0].ServiceTarget).Should(Equal("https://10.0.0.27:2333"))
		Expect(exposure[0].PathPrefix).Should(Equal("/"))
		Expect(exposure[0].IsDeleted).Should(BeFalse())
		Expect(exposure[0].ProxySSLVerifyEnabled).Should(BeNil())
	})

	It("should resolve https with proxy-ssl-verify disabled", func() {
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
				ClusterIP: "10.0.0.28",
				Ports: []v1.ServicePort{
					{
						Name:     "https",
						Protocol: v1.ProtocolTCP,
						Port:     2333,
						TargetPort: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: 443,
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
				Annotations: map[string]string{
					"cloudflare-tunnel-ingress-controller.strrl.dev/backend-protocol": "https",
					"cloudflare-tunnel-ingress-controller.strrl.dev/proxy-ssl-verify": "off",
				},
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
													Name: "https",
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

		By("transforming ingress to exposure")
		exposure, err := controller.FromIngressToExposure(ctx, logger, kubeClient, ingress)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(exposure).ShouldNot(BeNil())
		Expect(exposure).Should(HaveLen(1))
		Expect(exposure[0].Hostname).Should(Equal("test.example.com"))
		Expect(exposure[0].ServiceTarget).Should(Equal("https://10.0.0.28:2333"))
		Expect(exposure[0].PathPrefix).Should(Equal("/"))
		Expect(exposure[0].IsDeleted).Should(BeFalse())
		Expect(exposure[0].ProxySSLVerifyEnabled).ShouldNot(BeNil())
		Expect(*exposure[0].ProxySSLVerifyEnabled).Should(BeFalse())

	})
	It("should resolve https with proxy-ssl-verify enabled", func() {
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
				ClusterIP: "10.0.0.29",
				Ports: []v1.ServicePort{
					{
						Name:     "https",
						Protocol: v1.ProtocolTCP,
						Port:     2333,
						TargetPort: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: 443,
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
				Annotations: map[string]string{
					"cloudflare-tunnel-ingress-controller.strrl.dev/backend-protocol": "https",
					"cloudflare-tunnel-ingress-controller.strrl.dev/proxy-ssl-verify": "on",
				},
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
													Name: "https",
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

		By("transforming ingress to exposure")
		exposure, err := controller.FromIngressToExposure(ctx, logger, kubeClient, ingress)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(exposure).ShouldNot(BeNil())
		Expect(exposure).Should(HaveLen(1))
		Expect(exposure[0].Hostname).Should(Equal("test.example.com"))
		Expect(exposure[0].ServiceTarget).Should(Equal("https://10.0.0.29:2333"))
		Expect(exposure[0].PathPrefix).Should(Equal("/"))
		Expect(exposure[0].IsDeleted).Should(BeFalse())
		Expect(exposure[0].ProxySSLVerifyEnabled).ShouldNot(BeNil())
		Expect(*exposure[0].ProxySSLVerifyEnabled).Should(BeTrue())
	})

	It("should update service load balancer ingress hostname", func() {
		// prepare
		By("preparing namespace")
		namespaceFixtures := fixtures.NewKubernetesNamespaceFixtures(IntegrationTestNamespace, kubeClient)
		ns, err := namespaceFixtures.Start(ctx)
		Expect(err).ShouldNot(HaveOccurred())
		expectedHostname := "test.example.com"

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
						Host: expectedHostname,
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

		By("transforming ingress to exposure")
		exposure, err := controller.FromIngressToExposure(ctx, logger, kubeClient, ingress)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(exposure).ShouldNot(BeNil())

		err = kubeClient.Get(ctx, types.NamespacedName{
			Namespace: ns,
			Name:      service.Name,
		}, &service)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(service.Status.LoadBalancer.Ingress).Should(ContainElement(v1.LoadBalancerIngress{Hostname: expectedHostname}))
	})

	It("should support multiple service backends", func() {
		// prepare
		By("preparing namespace")
		namespaceFixtures := fixtures.NewKubernetesNamespaceFixtures(IntegrationTestNamespace, kubeClient)
		ns, err := namespaceFixtures.Start(ctx)
		Expect(err).ShouldNot(HaveOccurred())
		expectedHostname1 := "test1.example.com"
		expectedHostname2 := "test2.example.com"

		defer func() {
			By("cleaning up namespace")
			err := namespaceFixtures.Stop(ctx)
			Expect(err).ShouldNot(HaveOccurred())
		}()

		By("preparing service 1")
		service1 := v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    ns,
				GenerateName: "test-service1-",
			},
			Spec: v1.ServiceSpec{
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
		err = kubeClient.Create(ctx, &service1)
		Expect(err).ShouldNot(HaveOccurred())

		By("preparing service 2")
		service2 := v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    ns,
				GenerateName: "test-service2-",
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{
						Name:     "http",
						Protocol: v1.ProtocolTCP,
						Port:     2334,
						TargetPort: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: 8088,
						},
					},
				},
			},
		}
		err = kubeClient.Create(ctx, &service2)
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
						Host: expectedHostname1,
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path:     "/",
										PathType: &pathTypePrefix,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: service1.Name,
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
					{
						Host: expectedHostname2,
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path:     "/",
										PathType: &pathTypePrefix,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: service2.Name,
												Port: networkingv1.ServiceBackendPort{
													Number: 2334,
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

		By("transforming ingress to exposure")
		exposure, err := controller.FromIngressToExposure(ctx, logger, kubeClient, ingress)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(exposure).ShouldNot(BeNil())

		err = kubeClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: service1.Name}, &service1)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(service1.Status.LoadBalancer.Ingress).Should(ContainElement(v1.LoadBalancerIngress{Hostname: expectedHostname1}))

		err = kubeClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: service2.Name}, &service2)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(service2.Status.LoadBalancer.Ingress).Should(ContainElement(v1.LoadBalancerIngress{Hostname: expectedHostname2}))
	})

	It("should support multiple hosts on a single service", func() {
		// prepare
		By("preparing namespace")
		namespaceFixtures := fixtures.NewKubernetesNamespaceFixtures(IntegrationTestNamespace, kubeClient)
		ns, err := namespaceFixtures.Start(ctx)
		Expect(err).ShouldNot(HaveOccurred())
		expectedHostname1 := "test1.example.com"
		expectedHostname2 := "test2.example.com"

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
						Host: expectedHostname1,
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
					{
						Host: expectedHostname2,
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
													Number: 2334,
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

		By("transforming ingress to exposure")
		exposure, err := controller.FromIngressToExposure(ctx, logger, kubeClient, ingress)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(exposure).ShouldNot(BeNil())

		err = kubeClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: service.Name}, &service)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(service.Status.LoadBalancer.Ingress).Should(ContainElement(v1.LoadBalancerIngress{Hostname: expectedHostname1}))
		Expect(service.Status.LoadBalancer.Ingress).Should(ContainElement(v1.LoadBalancerIngress{Hostname: expectedHostname2}))
	})
})
