package controller

import (
	"reflect"
	"testing"
	"time"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/apis/cloudflareaccess/v1alpha1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ingressWithHosts(hosts ...string) networkingv1.Ingress {
	ingress := networkingv1.Ingress{}
	for _, host := range hosts {
		ingress.Spec.Rules = append(ingress.Spec.Rules, networkingv1.IngressRule{Host: host})
	}
	return ingress
}

func Test_collectHostnames(t *testing.T) {
	tests := []struct {
		name      string
		ingresses []networkingv1.Ingress
		want      []string
	}{
		{
			name:      "no ingresses",
			ingresses: nil,
			want:      nil,
		},
		{
			name:      "skip rules without host",
			ingresses: []networkingv1.Ingress{ingressWithHosts("", "app.example.com")},
			want:      []string{"app.example.com"},
		},
		{
			name: "deduplicate and sort across ingresses",
			ingresses: []networkingv1.Ingress{
				ingressWithHosts("b.example.com", "a.example.com"),
				ingressWithHosts("a.example.com", "c.example.com"),
			},
			want: []string{"a.example.com", "b.example.com", "c.example.com"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectHostnames(tt.ingresses)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collectHostnames() = %v, want %v", got, tt.want)
			}
		})
	}
}

func accessWithAge(namespace string, name string, created time.Time) *v1alpha1.CloudflareAccess {
	access := &v1alpha1.CloudflareAccess{}
	access.Namespace = namespace
	access.Name = name
	access.CreationTimestamp = metav1.NewTime(created)
	return access
}

func Test_accessPrecedes(t *testing.T) {
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)

	tests := []struct {
		name string
		a    *v1alpha1.CloudflareAccess
		b    *v1alpha1.CloudflareAccess
		want bool
	}{
		{
			name: "older object wins",
			a:    accessWithAge("ns", "a", older),
			b:    accessWithAge("ns", "b", newer),
			want: true,
		},
		{
			name: "newer object loses",
			a:    accessWithAge("ns", "a", newer),
			b:    accessWithAge("ns", "b", older),
			want: false,
		},
		{
			name: "tie breaks by namespace and name",
			a:    accessWithAge("ns-a", "same", older),
			b:    accessWithAge("ns-b", "same", older),
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := accessPrecedes(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("accessPrecedes() = %v, want %v", got, tt.want)
			}
		})
	}
}
