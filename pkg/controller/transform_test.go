package controller

import (
	"context"
	"testing"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetHostFromService(t *testing.T) {
	for _, tc := range []struct {
		name    string
		service *v1.Service
		want    string
		wantErr bool
	}{
		{
			name: "cluster_ip",
			service: &v1.Service{
				Spec: v1.ServiceSpec{
					ClusterIP: "1.1.1.1",
				},
			},
			want: "1.1.1.1",
		},
		{
			name: "headless",
			service: &v1.Service{
				Spec: v1.ServiceSpec{
					ClusterIP: "None",
				},
			},
			wantErr: true,
		},
		{
			name: "external_name",
			service: &v1.Service{
				Spec: v1.ServiceSpec{
					ExternalName: "example.default.svc.cluster.local",
				},
			},
			want: "example.default.svc.cluster.local",
		},
		{
			name: "empty",
			service: &v1.Service{
				Spec: v1.ServiceSpec{},
			},
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := getHostFromService(tc.service)

			if got != tc.want {
				t.Errorf("getHostFromService() = %q, want %q", got, tc.want)
			}

			if (err != nil) != tc.wantErr {
				t.Errorf("getHostFromService() returns unexpected error: %v", err)
			}
		})
	}
}

func TestFromIngressToExposure(t *testing.T) {
	type args struct {
		ingress networkingv1.Ingress
	}
	type wants struct {
		exposures []exposure.Exposure
		wantErr   bool
	}
	tests := []struct {
		name    string
		args    args
		wants   wants
		objects []client.Object
	}{
		{
			name: "should handle multiple rules",
			args: args{
				ingress: networkingv1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-ingress",
						Namespace: "default",
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								Host: "foo.example.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path: "/",
												PathType: func() *networkingv1.PathType {
													pt := networkingv1.PathTypePrefix
													return &pt
												}(),
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "foo-service",
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
							{
								Host: "bar.example.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path: "/",
												PathType: func() *networkingv1.PathType {
													pt := networkingv1.PathTypePrefix
													return &pt
												}(),
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "bar-service",
														Port: networkingv1.ServiceBackendPort{
															Number: 8080,
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
				},
			},
			wants: wants{
				exposures: []exposure.Exposure{
					{
						Hostname:      "foo.example.com",
						ServiceTarget: "http://10.0.0.1:80",
						PathPrefix:    "/",
						AccessApplicationName: func() *string {
							s := "test-ingress"
							return &s
						}(),
						AccessPolicyAllowedEmails: DefaultAccessPolicyAllowedEmails,
					},
					{
						Hostname:      "bar.example.com",
						ServiceTarget: "http://10.0.0.2:8080",
						PathPrefix:    "/",
						AccessApplicationName: func() *string {
							s := "test-ingress"
							return &s
						}(),
						AccessPolicyAllowedEmails: DefaultAccessPolicyAllowedEmails,
					},
				},
			},
			objects: []client.Object{
				&v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo-service",
						Namespace: "default",
					},
					Spec: v1.ServiceSpec{
						ClusterIP: "10.0.0.1",
						Ports: []v1.ServicePort{
							{
								Port: 80,
							},
						},
					},
				},
				&v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bar-service",
						Namespace: "default",
					},
					Spec: v1.ServiceSpec{
						ClusterIP: "10.0.0.2",
						Ports: []v1.ServicePort{
							{
								Port: 8080,
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, v1.AddToScheme(scheme))
			require.NoError(t, networkingv1.AddToScheme(scheme))

			kubeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				Build()

			got, err := FromIngressToExposure(context.Background(), logr.Discard(), kubeClient, tt.args.ingress)
			if tt.wants.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wants.exposures, got)
			}
		})
	}
}
