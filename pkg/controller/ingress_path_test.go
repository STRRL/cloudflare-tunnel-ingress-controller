package controller

import (
	"context"
	"regexp"
	"testing"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestFromIngressToExposurePathMatching(t *testing.T) {
	for _, tc := range []struct {
		name       string
		path       string
		pathType   networkingv1.PathType
		matches    []string
		nonMatches []string
	}{
		{
			name:       "prefix matches whole path elements at the start",
			path:       "/wp-admin",
			pathType:   networkingv1.PathTypePrefix,
			matches:    []string{"/wp-admin", "/wp-admin/", "/wp-admin/edit.php"},
			nonMatches: []string{"/foo/wp-admin", "/wp-admin-other", "/wp-administrator", "/WP-ADMIN", "/"},
		},
		{
			name:       "prefix ignores trailing slash",
			path:       "/wp-admin/",
			pathType:   networkingv1.PathTypePrefix,
			matches:    []string{"/wp-admin", "/wp-admin/", "/wp-admin/edit.php"},
			nonMatches: []string{"/foo/wp-admin/", "/wp-admin-other/"},
		},
		{
			name:       "nested prefix matches whole path elements",
			path:       "/foo/bar",
			pathType:   networkingv1.PathTypePrefix,
			matches:    []string{"/foo/bar", "/foo/bar/", "/foo/bar/baz"},
			nonMatches: []string{"/foo/barbaz", "/foo/bars", "/other/foo/bar"},
		},
		{
			name:     "root prefix matches all absolute paths",
			path:     "/",
			pathType: networkingv1.PathTypePrefix,
			matches:  []string{"/", "/wp-admin", "/foo/wp-admin"},
		},
		{
			name:       "prefix escapes regex metacharacters",
			path:       "/v1.0/app+name",
			pathType:   networkingv1.PathTypePrefix,
			matches:    []string{"/v1.0/app+name", "/v1.0/app+name/child"},
			nonMatches: []string{"/v1X0/appname", "/v1.0/apppname", "/v1.0/app+names", "/other/v1.0/app+name"},
		},
		{
			name:       "prefix keeps regex groups and alternatives literal",
			path:       "/foo(bar)|baz",
			pathType:   networkingv1.PathTypePrefix,
			matches:    []string{"/foo(bar)|baz", "/foo(bar)|baz/child"},
			nonMatches: []string{"/foobar", "/baz", "/foo(bar)|bazooka"},
		},
		{
			name:       "implementation specific retains unanchored matching",
			path:       "/wp-admin",
			pathType:   networkingv1.PathTypeImplementationSpecific,
			matches:    []string{"/wp-admin", "/foo/wp-admin", "/wp-admin-other"},
			nonMatches: []string{"/WP-ADMIN", "/"},
		},
		{
			name:       "implementation specific retains regex syntax",
			path:       "/(admin|login)$",
			pathType:   networkingv1.PathTypeImplementationSpecific,
			matches:    []string{"/admin", "/login", "/foo/admin"},
			nonMatches: []string{"/admin/edit", "/login-other", "/ADMIN"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &v1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
				Spec:       v1.ServiceSpec{ClusterIP: "10.0.0.1", Ports: []v1.ServicePort{{Port: 80}}},
			}
			kubeClient := fake.NewClientBuilder().WithObjects(service).Build()
			ingress := networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{Name: "path-matching", Namespace: "default"},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{
						Host: "example.com",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{{
									Path: tc.path, PathType: ptr.To(tc.pathType),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "app", Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								}},
							},
						},
					}},
				},
			}
			exposures, err := FromIngressToExposure(context.Background(), logr.Discard(), kubeClient,
				record.NewFakeRecorder(8), ingress, "cluster.local")
			if err != nil {
				t.Fatal(err)
			}
			if len(exposures) != 1 {
				t.Fatalf("expected one exposure, got %d", len(exposures))
			}
			if tc.pathType == networkingv1.PathTypeImplementationSpecific && exposures[0].PathPrefix != tc.path {
				t.Fatalf("ImplementationSpecific path changed: got %q, want %q", exposures[0].PathPrefix, tc.path)
			}
			// Cloudflare interprets the generated path as a Go regular expression.
			pattern, err := regexp.Compile(exposures[0].PathPrefix)
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range tc.matches {
				if !pattern.MatchString(path) {
					t.Errorf("path %q should match, generated regex %q", path, pattern)
				}
			}
			for _, path := range tc.nonMatches {
				if pattern.MatchString(path) {
					t.Errorf("path %q should not match, generated regex %q", path, pattern)
				}
			}
		})
	}
}
