package controller

import (
	"testing"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
