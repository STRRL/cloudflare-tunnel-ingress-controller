package controller

import (
	"testing"

	"k8s.io/api/core/v1"
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
