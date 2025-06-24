package e2e

import (
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var pathTypePrefix = networkingv1.PathTypePrefix

// logProgress uses Ginkgo's immediate logging
func logProgress(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	GinkgoLogr.Info(msg)
}

var _ = Describe("Full Workflow E2E Tests", func() {
	var (
		testNamespace string
		testHostname  string
		cleanup       []func() error
	)

	BeforeEach(func() {
		testNamespace = fmt.Sprintf("e2e-test-%d", time.Now().Unix())
		testHostname = GenerateTestHostname()
		cleanup = []func() error{}

		logProgress("\n=== E2E Test Setup ===\n")
		logProgress("Generated test namespace: %s\n", testNamespace)
		logProgress("Generated test hostname: %s\n", testHostname)
		logProgress("Configuration loaded from .env file\n")
		
		By(fmt.Sprintf("using test namespace: %s", testNamespace))
		By(fmt.Sprintf("using test hostname: %s", testHostname))
	})

	AfterEach(func() {
		logProgress("\n=== E2E Test Cleanup ===\n")
		By("cleaning up resources")
		logProgress("Running %d cleanup functions...\n", len(cleanup))
		for i := len(cleanup) - 1; i >= 0; i-- {
			if err := cleanup[i](); err != nil {
				logProgress("⚠ Cleanup function %d failed: %v\n", i, err)
				GinkgoLogr.Error(err, "cleanup failed")
			} else {
				logProgress("✓ Cleanup function %d completed\n", i)
			}
		}
		logProgress("=== E2E Test Cleanup Complete ===\n")
	})

	It("should create ingress and make domain accessible", func() {
		logProgress("=== Starting E2E Test ===\n")
		logProgress("Test namespace: %s\n", testNamespace)
		logProgress("Test hostname: %s\n", testHostname)
		
		By("creating test namespace")
		logProgress("Creating namespace: %s\n", testNamespace)
		ns := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		}
		Expect(kubeClient.Create(ctx, ns)).To(Succeed())
		cleanup = append(cleanup, func() error {
			return kubeClient.Delete(ctx, ns)
		})
		logProgress("✓ Namespace created successfully\n")

		By("creating backend service")
		logProgress("Creating backend service...\n")
		service := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-backend",
				Namespace: testNamespace,
			},
			Spec: v1.ServiceSpec{
				Selector: map[string]string{
					"app": "test-backend",
				},
				Ports: []v1.ServicePort{
					{
						Name:     "http",
						Protocol: v1.ProtocolTCP,
						Port:     80,
						TargetPort: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: 80,
						},
					},
				},
			},
		}
		Expect(kubeClient.Create(ctx, service)).To(Succeed())
		logProgress("✓ Backend service created successfully\n")

		By("creating backend deployment")
		logProgress("Creating backend deployment...\n")
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-backend",
				Namespace: testNamespace,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: int32Ptr(1),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test-backend",
					},
				},
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "test-backend",
						},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "nginx",
								Image: "nginx:alpine",
								Ports: []v1.ContainerPort{
									{
										ContainerPort: 80,
									},
								},
								Env: []v1.EnvVar{
									{
										Name:  "NGINX_PORT",
										Value: "80",
									},
								},
								VolumeMounts: []v1.VolumeMount{
									{
										Name:      "html",
										MountPath: "/usr/share/nginx/html",
									},
								},
							},
						},
						Volumes: []v1.Volume{
							{
								Name: "html",
								VolumeSource: v1.VolumeSource{
									ConfigMap: &v1.ConfigMapVolumeSource{
										LocalObjectReference: v1.LocalObjectReference{
											Name: "test-html",
										},
									},
								},
							},
						},
					},
				},
			},
		}
		Expect(kubeClient.Create(ctx, deployment)).To(Succeed())
		logProgress("✓ Backend deployment created successfully\n")

		By("creating HTML content configmap")
		logProgress("Creating HTML content configmap...\n")
		configMap := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-html",
				Namespace: testNamespace,
			},
			Data: map[string]string{
				"index.html": fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>E2E Test</title></head>
<body>
<h1>E2E Test Success</h1>
<p>Hostname: %s</p>
<p>Namespace: %s</p>
<p>Timestamp: %s</p>
</body>
</html>`, testHostname, testNamespace, time.Now().Format(time.RFC3339)),
			},
		}
		Expect(kubeClient.Create(ctx, configMap)).To(Succeed())
		logProgress("✓ HTML content configmap created successfully\n")

		By("waiting for deployment to be ready")
		logProgress("Waiting for backend deployment to be ready...\n")
		Eventually(func() bool {
			var dep appsv1.Deployment
			err := kubeClient.Get(ctx, types.NamespacedName{
				Namespace: testNamespace,
				Name:      "test-backend",
			}, &dep)
			if err != nil {
				return false
			}
			return dep.Status.ReadyReplicas == 1
		}, time.Minute*2, time.Second*5).Should(BeTrue())
		logProgress("✓ Backend deployment is ready\n")

		By("creating ingress resource")
		logProgress("Creating ingress resource for hostname: %s\n", testHostname)
		ingress := &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-ingress",
				Namespace: testNamespace,
				Annotations: map[string]string{
					"kubernetes.io/ingress.class": "cloudflare-tunnel",
				},
			},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{
					{
						Host: testHostname,
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path:     "/",
										PathType: &pathTypePrefix,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "test-backend",
												Port: networkingv1.ServiceBackendPort{
													Number: 80,
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
		Expect(kubeClient.Create(ctx, ingress)).To(Succeed())
		cleanup = append(cleanup, func() error {
			return kubeClient.Delete(ctx, ingress)
		})
		logProgress("✓ Ingress resource created successfully\n")

		By("waiting for ingress to be processed by controller")
		logProgress("Waiting for controller to process the ingress (up to 5 minutes)...\n")
		Eventually(func() bool {
			var ing networkingv1.Ingress
			err := kubeClient.Get(ctx, types.NamespacedName{
				Namespace: testNamespace,
				Name:      "test-ingress",
			}, &ing)
			if err != nil {
				return false
			}
			// Check if ingress has LoadBalancer status
			return len(ing.Status.LoadBalancer.Ingress) > 0
		}, time.Minute*5, time.Second*10).Should(BeTrue())
		logProgress("✓ Controller has processed the ingress (LoadBalancer status set)\n")

		By("verifying domain accessibility")
		logProgress("Testing HTTP accessibility for: http://%s (up to 10 minutes)...\n", testHostname)
		Eventually(func() error {
			client := &http.Client{
				Timeout: time.Second * 30,
			}

			resp, err := client.Get(fmt.Sprintf("http://%s", testHostname))
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("expected status 200, got %d", resp.StatusCode)
			}

			return nil
		}, time.Minute*10, time.Second*15).Should(Succeed())
		logProgress("✓ HTTP accessibility verified successfully\n")

		By("verifying HTTPS accessibility")
		logProgress("Testing HTTPS accessibility for: https://%s (up to 5 minutes)...\n", testHostname)
		Eventually(func() error {
			client := &http.Client{
				Timeout: time.Second * 30,
			}

			resp, err := client.Get(fmt.Sprintf("https://%s", testHostname))
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("expected status 200, got %d", resp.StatusCode)
			}

			return nil
		}, time.Minute*5, time.Second*15).Should(Succeed())
		logProgress("✓ HTTPS accessibility verified successfully\n")

		By("deleting ingress and verifying cleanup")
		logProgress("Deleting ingress to test cleanup...\n")
		Expect(kubeClient.Delete(ctx, ingress)).To(Succeed())

		// Remove the ingress cleanup since we already deleted it
		if len(cleanup) > 0 && cleanup[len(cleanup)-1] != nil {
			cleanup = cleanup[:len(cleanup)-1]
		}
		logProgress("✓ Ingress deleted successfully\n")

		By("waiting for domain to become inaccessible")
		logProgress("Waiting for domain to become inaccessible (up to 5 minutes)...\n")
		Eventually(func() bool {
			client := &http.Client{
				Timeout: time.Second * 10,
			}

			resp, err := client.Get(fmt.Sprintf("https://%s", testHostname))
			if err != nil {
				// Network error means domain is not accessible
				return true
			}
			defer resp.Body.Close()

			// Domain is still accessible
			return resp.StatusCode == http.StatusNotFound || resp.StatusCode >= 500
		}, time.Minute*5, time.Second*15).Should(BeTrue())
		logProgress("✓ Domain cleanup verified - domain is no longer accessible\n")
		logProgress("=== E2E Test Completed Successfully ===\n")
	})

	It("should create ingress with ingressClassName and make domain accessible", func() {
		logProgress("=== Starting E2E Test with ingressClassName ===\n")
		logProgress("Test namespace: %s\n", testNamespace)
		logProgress("Test hostname: %s\n", testHostname)
		
		By("creating test namespace")
		logProgress("Creating namespace: %s\n", testNamespace)
		ns := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		}
		Expect(kubeClient.Create(ctx, ns)).To(Succeed())
		cleanup = append(cleanup, func() error {
			return kubeClient.Delete(ctx, ns)
		})
		logProgress("✓ Namespace created successfully\n")

		By("creating backend service")
		logProgress("Creating backend service...\n")
		service := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-backend",
				Namespace: testNamespace,
			},
			Spec: v1.ServiceSpec{
				Selector: map[string]string{
					"app": "test-backend",
				},
				Ports: []v1.ServicePort{
					{
						Name:     "http",
						Protocol: v1.ProtocolTCP,
						Port:     80,
						TargetPort: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: 80,
						},
					},
				},
			},
		}
		Expect(kubeClient.Create(ctx, service)).To(Succeed())
		logProgress("✓ Backend service created successfully\n")

		By("creating backend deployment")
		logProgress("Creating backend deployment...\n")
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-backend",
				Namespace: testNamespace,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: int32Ptr(1),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test-backend",
					},
				},
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "test-backend",
						},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "nginx",
								Image: "nginx:alpine",
								Ports: []v1.ContainerPort{
									{
										ContainerPort: 80,
									},
								},
								Env: []v1.EnvVar{
									{
										Name:  "NGINX_PORT",
										Value: "80",
									},
								},
								VolumeMounts: []v1.VolumeMount{
									{
										Name:      "html",
										MountPath: "/usr/share/nginx/html",
									},
								},
							},
						},
						Volumes: []v1.Volume{
							{
								Name: "html",
								VolumeSource: v1.VolumeSource{
									ConfigMap: &v1.ConfigMapVolumeSource{
										LocalObjectReference: v1.LocalObjectReference{
											Name: "test-html",
										},
									},
								},
							},
						},
						RestartPolicy: v1.RestartPolicyAlways,
					},
				},
			},
		}
		Expect(kubeClient.Create(ctx, deployment)).To(Succeed())
		logProgress("✓ Backend deployment created successfully\n")

		By("creating HTML content configmap")
		logProgress("Creating HTML content configmap...\n")
		configMap := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-html",
				Namespace: testNamespace,
			},
			Data: map[string]string{
				"index.html": fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>E2E Test with ingressClassName</title></head>
<body>
<h1>E2E Test Success (ingressClassName)</h1>
<p>Hostname: %s</p>
<p>Namespace: %s</p>
<p>Timestamp: %s</p>
<p>Method: spec.ingressClassName</p>
</body>
</html>`, testHostname, testNamespace, time.Now().Format(time.RFC3339)),
			},
		}
		Expect(kubeClient.Create(ctx, configMap)).To(Succeed())
		logProgress("✓ HTML content configmap created successfully\n")

		By("waiting for deployment to be ready")
		logProgress("Waiting for backend deployment to be ready...\n")
		Eventually(func() bool {
			var dep appsv1.Deployment
			err := kubeClient.Get(ctx, types.NamespacedName{
				Namespace: testNamespace,
				Name:      "test-backend",
			}, &dep)
			if err != nil {
				return false
			}
			return dep.Status.ReadyReplicas == 1
		}, time.Minute*2, time.Second*5).Should(BeTrue())
		logProgress("✓ Backend deployment is ready\n")

		By("creating ingress resource with ingressClassName")
		logProgress("Creating ingress with ingressClassName for hostname: %s\n", testHostname)
		ingressClassName := "cloudflare-tunnel"
		ingress := &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-ingress",
				Namespace: testNamespace,
			},
			Spec: networkingv1.IngressSpec{
				IngressClassName: &ingressClassName,
				Rules: []networkingv1.IngressRule{
					{
						Host: testHostname,
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path:     "/",
										PathType: &pathTypePrefix,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "test-backend",
												Port: networkingv1.ServiceBackendPort{
													Number: 80,
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
		Expect(kubeClient.Create(ctx, ingress)).To(Succeed())
		cleanup = append(cleanup, func() error {
			return kubeClient.Delete(ctx, ingress)
		})
		logProgress("✓ Ingress resource with ingressClassName created successfully\n")

		By("waiting for ingress to be processed by controller")
		logProgress("Waiting for controller to process the ingress (up to 5 minutes)...\n")
		Eventually(func() bool {
			var ing networkingv1.Ingress
			err := kubeClient.Get(ctx, types.NamespacedName{
				Namespace: testNamespace,
				Name:      "test-ingress",
			}, &ing)
			if err != nil {
				return false
			}
			// Check if ingress has LoadBalancer status
			return len(ing.Status.LoadBalancer.Ingress) > 0
		}, time.Minute*5, time.Second*10).Should(BeTrue())
		logProgress("✓ Controller has processed the ingress (LoadBalancer status set)\n")

		By("verifying domain accessibility")
		logProgress("Testing HTTP accessibility for: http://%s (up to 10 minutes)...\n", testHostname)
		Eventually(func() error {
			client := &http.Client{
				Timeout: time.Second * 30,
			}
			
			resp, err := client.Get(fmt.Sprintf("http://%s", testHostname))
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("expected status 200, got %d", resp.StatusCode)
			}

			return nil
		}, time.Minute*10, time.Second*15).Should(Succeed())
		logProgress("✓ HTTP accessibility verified successfully\n")

		By("verifying HTTPS accessibility")
		logProgress("Testing HTTPS accessibility for: https://%s (up to 5 minutes)...\n", testHostname)
		Eventually(func() error {
			client := &http.Client{
				Timeout: time.Second * 30,
			}
			
			resp, err := client.Get(fmt.Sprintf("https://%s", testHostname))
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("expected status 200, got %d", resp.StatusCode)
			}

			return nil
		}, time.Minute*5, time.Second*15).Should(Succeed())
		logProgress("✓ HTTPS accessibility verified successfully\n")

		By("deleting ingress and verifying cleanup")
		logProgress("Deleting ingress to test cleanup...\n")
		Expect(kubeClient.Delete(ctx, ingress)).To(Succeed())

		// Remove the ingress cleanup since we already deleted it
		if len(cleanup) > 0 && cleanup[len(cleanup)-1] != nil {
			cleanup = cleanup[:len(cleanup)-1]
		}
		logProgress("✓ Ingress deleted successfully\n")

		By("waiting for domain to become inaccessible")
		logProgress("Waiting for domain to become inaccessible (up to 5 minutes)...\n")
		Eventually(func() bool {
			client := &http.Client{
				Timeout: time.Second * 10,
			}
			
			resp, err := client.Get(fmt.Sprintf("https://%s", testHostname))
			if err != nil {
				// Network error means domain is not accessible
				return true
			}
			defer resp.Body.Close()
			
			// Domain is still accessible
			return resp.StatusCode == http.StatusNotFound || resp.StatusCode >= 500
		}, time.Minute*5, time.Second*15).Should(BeTrue())
		logProgress("✓ Domain cleanup verified - domain is no longer accessible\n")
		logProgress("=== E2E Test with ingressClassName Completed Successfully ===\n")
	})
})

func int32Ptr(i int32) *int32 {
	return &i
}
