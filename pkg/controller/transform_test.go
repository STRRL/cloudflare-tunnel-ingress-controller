package controller

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetHostFromService(t *testing.T) {
	for _, tc := range []struct {
		name          string
		service       *v1.Service
		clusterDomain string
		want          string
		wantErr       bool
	}{
		{
			name: "cluster_ip_service",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-service",
					Namespace: "default",
				},
				Spec: v1.ServiceSpec{
					ClusterIP: "1.1.1.1",
				},
			},
			clusterDomain: "cluster.local",
			want:          "my-service.default.svc.cluster.local",
		},
		{
			name: "headless",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "headless-service",
					Namespace: "default",
				},
				Spec: v1.ServiceSpec{
					ClusterIP: "None",
				},
			},
			clusterDomain: "cluster.local",
			wantErr:       true,
		},
		{
			name: "external_name",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "external-service",
					Namespace: "default",
				},
				Spec: v1.ServiceSpec{
					Type:         v1.ServiceTypeExternalName,
					ExternalName: "example.default.svc.cluster.local",
				},
			},
			clusterDomain: "cluster.local",
			want:          "example.default.svc.cluster.local",
		},
		{
			name: "different_namespace",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app-service",
					Namespace: "production",
				},
				Spec: v1.ServiceSpec{
					ClusterIP: "10.0.0.1",
				},
			},
			clusterDomain: "cluster.local",
			want:          "app-service.production.svc.cluster.local",
		},
		{
			name: "custom_cluster_domain",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-service",
					Namespace: "default",
				},
				Spec: v1.ServiceSpec{
					ClusterIP: "10.0.0.5",
				},
			},
			clusterDomain: "my-custom.domain",
			want:          "my-service.default.svc.my-custom.domain",
		},
		{
			name: "custom_cluster_domain_different_namespace",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "backend",
					Namespace: "staging",
				},
				Spec: v1.ServiceSpec{
					ClusterIP: "10.0.0.6",
				},
			},
			clusterDomain: "k8s.internal",
			want:          "backend.staging.svc.k8s.internal",
		},
		{
			name: "external_name_ignores_cluster_domain",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "external-service",
					Namespace: "default",
				},
				Spec: v1.ServiceSpec{
					Type:         v1.ServiceTypeExternalName,
					ExternalName: "api.example.com",
				},
			},
			clusterDomain: "my-custom.domain",
			want:          "api.example.com",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := getHostFromService(tc.service, tc.clusterDomain)

			if got != tc.want {
				t.Errorf("getHostFromService() = %q, want %q", got, tc.want)
			}

			if (err != nil) != tc.wantErr {
				t.Errorf("getHostFromService() returns unexpected error: %v", err)
			}
		})
	}
}

func TestFromIngressToExposureNilHTTP(t *testing.T) {
	// rule.HTTP is optional in the Ingress API, a rule may carry only a
	// host. Previously this dereferenced the nil pointer and panicked;
	// such rules must be skipped without failing the whole ingress.
	ingress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "host-only",
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: nil,
					},
				},
			},
		},
	}

	exposures, err := FromIngressToExposure(context.Background(), logr.Discard(), nil, ingress, "cluster.local")
	if err != nil {
		t.Fatalf("expected a rule with nil HTTP to be skipped, got error: %v", err)
	}
	if len(exposures) != 0 {
		t.Fatalf("expected no exposures for a rule with nil HTTP, got %d", len(exposures))
	}
}

func TestFromIngressToExposureNilHTTPKeepsOtherRules(t *testing.T) {
	// A host-only rule must not drop the valid rules of the same ingress.
	pathType := networkingv1.PathTypePrefix
	ingress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "mixed-rules",
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "app.example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "my-app",
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								},
							},
						},
					},
				},
				{
					Host: "placeholder.example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: nil,
					},
				},
			},
		},
	}

	service := v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "my-app",
		},
		Spec: v1.ServiceSpec{
			ClusterIP: "10.0.0.1",
			Ports:     []v1.ServicePort{{Port: 80}},
		},
	}
	kubeClient := fake.NewClientBuilder().WithObjects(&service).Build()

	exposures, err := FromIngressToExposure(context.Background(), logr.Discard(), kubeClient, ingress, "cluster.local")
	if err != nil {
		t.Fatalf("expected the host-only rule to be skipped, got error: %v", err)
	}
	if len(exposures) != 1 {
		t.Fatalf("expected exactly the valid rule to be exposed, got %d exposures", len(exposures))
	}
	if exposures[0].Hostname != "app.example.com" {
		t.Fatalf("expected exposure for app.example.com, got %s", exposures[0].Hostname)
	}
}
