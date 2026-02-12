package cloudflarecontroller

import (
	"context"
	"reflect"
	"testing"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/cloudflare/cloudflare-go"
	"k8s.io/utils/ptr"
)

func Test_fromExposureToCloudflareIngress(t *testing.T) {
	type args struct {
		ctx      context.Context
		exposure exposure.Exposure
	}
	tests := []struct {
		name    string
		args    args
		want    *cloudflare.UnvalidatedIngressRule
		wantErr bool
	}{
		{
			name: "deleted exposure",
			args: args{
				ctx: context.Background(),
				exposure: exposure.Exposure{
					IsDeleted: true,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "valid exposure",
			args: args{
				ctx: context.Background(),
				exposure: exposure.Exposure{
					Hostname:      "ingress.example.com",
					ServiceTarget: "http://10.0.0.1:80",
					PathPrefix:    "/",
					IsDeleted:     false,
				},
			},
			want: &cloudflare.UnvalidatedIngressRule{
				Hostname:      "ingress.example.com",
				Path:          "/",
				Service:       "http://10.0.0.1:80",
				OriginRequest: nil,
			},
			wantErr: false,
		},
		{
			name: "contains path",
			args: args{
				ctx: context.Background(),
				exposure: exposure.Exposure{
					Hostname:      "ingress.example.com",
					ServiceTarget: "http://10.0.0.1:80",
					PathPrefix:    "/prefix",
					IsDeleted:     false,
				},
			},
			want: &cloudflare.UnvalidatedIngressRule{
				Hostname:      "ingress.example.com",
				Path:          "/prefix",
				Service:       "http://10.0.0.1:80",
				OriginRequest: nil,
			},
			wantErr: false,
		},
		{
			name: "contains http-host-header",
			args: args{
				ctx: context.Background(),
				exposure: exposure.Exposure{
					Hostname:       "ingress.example.com",
					ServiceTarget:  "http://10.0.0.1:80",
					PathPrefix:     "/prefix",
					IsDeleted:      false,
					HTTPHostHeader: ptr.To("foo.internal"),
				},
			},
			want: &cloudflare.UnvalidatedIngressRule{
				Hostname: "ingress.example.com",
				Path:     "/prefix",
				Service:  "http://10.0.0.1:80",
				OriginRequest: &cloudflare.OriginRequestConfig{
					HTTPHostHeader: ptr.To("foo.internal"),
				},
			},
			wantErr: false,
		},
		{
			name: "https with origin-server-name",
			args: args{
				ctx: context.Background(),
				exposure: exposure.Exposure{
					Hostname:         "ingress.example.com",
					ServiceTarget:    "https://10.0.0.1:443",
					PathPrefix:       "/",
					IsDeleted:        false,
					OriginServerName: ptr.To("bar.internal"),
				},
			},
			want: &cloudflare.UnvalidatedIngressRule{
				Hostname: "ingress.example.com",
				Path:     "/",
				Service:  "https://10.0.0.1:443",
				OriginRequest: &cloudflare.OriginRequestConfig{
					NoTLSVerify:      boolPointer(true),
					OriginServerName: ptr.To("bar.internal"),
				},
			},
		},
		{
			name: "https with different http-host-header and origin-server-name",
			args: args{
				ctx: context.Background(),
				exposure: exposure.Exposure{
					Hostname:         "ingress.example.com",
					ServiceTarget:    "https://10.0.0.1:443",
					PathPrefix:       "/",
					IsDeleted:        false,
					HTTPHostHeader:   ptr.To("foo.internal"),
					OriginServerName: ptr.To("bar.internal"),
				},
			},
			want: &cloudflare.UnvalidatedIngressRule{
				Hostname: "ingress.example.com",
				Path:     "/",
				Service:  "https://10.0.0.1:443",
				OriginRequest: &cloudflare.OriginRequestConfig{
					NoTLSVerify:      boolPointer(true),
					HTTPHostHeader:   ptr.To("foo.internal"),
					OriginServerName: ptr.To("bar.internal"),
				},
			},
		}, {
			name: "https should enable no-tls-verify by default",
			args: args{
				ctx: context.Background(),
				exposure: exposure.Exposure{
					Hostname:      "ingress.example.com",
					ServiceTarget: "https://10.0.0.1:443",
					PathPrefix:    "/",
					IsDeleted:     false,
				},
			},
			want: &cloudflare.UnvalidatedIngressRule{
				Hostname: "ingress.example.com",
				Path:     "/",
				Service:  "https://10.0.0.1:443",
				OriginRequest: &cloudflare.OriginRequestConfig{
					NoTLSVerify: boolPointer(true),
				},
			},
		}, {
			name: "https with no-tls-verify enabled",
			args: args{
				ctx: context.Background(),
				exposure: exposure.Exposure{
					Hostname:              "ingress.example.com",
					ServiceTarget:         "https://10.0.0.1:443",
					PathPrefix:            "/",
					IsDeleted:             false,
					ProxySSLVerifyEnabled: boolPointer(false),
				},
			},
			want: &cloudflare.UnvalidatedIngressRule{
				Hostname: "ingress.example.com",
				Path:     "/",
				Service:  "https://10.0.0.1:443",
				OriginRequest: &cloudflare.OriginRequestConfig{
					NoTLSVerify: boolPointer(true),
				},
			},
		}, {
			name: "https with no-tls-verify disabled",
			args: args{
				ctx: context.Background(),
				exposure: exposure.Exposure{
					Hostname:              "ingress.example.com",
					ServiceTarget:         "https://10.0.0.1:443",
					PathPrefix:            "/",
					IsDeleted:             false,
					ProxySSLVerifyEnabled: boolPointer(true),
				},
			},
			want: &cloudflare.UnvalidatedIngressRule{
				Hostname: "ingress.example.com",
				Path:     "/",
				Service:  "https://10.0.0.1:443",
				OriginRequest: &cloudflare.OriginRequestConfig{
					NoTLSVerify: boolPointer(false),
				},
			},
		}, {
			name: "https with all options combined",
			args: args{
				ctx: context.Background(),
				exposure: exposure.Exposure{
					Hostname:              "ingress.example.com",
					ServiceTarget:         "https://my-svc.default.svc.cluster.local:8443",
					PathPrefix:            "/api",
					IsDeleted:             false,
					ProxySSLVerifyEnabled: boolPointer(true),
					HTTPHostHeader:        ptr.To("api.internal"),
					OriginServerName:      ptr.To("my-svc.internal"),
				},
			},
			want: &cloudflare.UnvalidatedIngressRule{
				Hostname: "ingress.example.com",
				Path:     "/api",
				Service:  "https://my-svc.default.svc.cluster.local:8443",
				OriginRequest: &cloudflare.OriginRequestConfig{
					NoTLSVerify:      boolPointer(false),
					HTTPHostHeader:   ptr.To("api.internal"),
					OriginServerName: ptr.To("my-svc.internal"),
				},
			},
		}, {
			name: "http backend does not set tls options",
			args: args{
				ctx: context.Background(),
				exposure: exposure.Exposure{
					Hostname:              "ingress.example.com",
					ServiceTarget:         "http://10.0.0.1:80",
					PathPrefix:            "/",
					IsDeleted:             false,
					ProxySSLVerifyEnabled: boolPointer(true),
					OriginServerName:      ptr.To("should-be-ignored"),
				},
			},
			want: &cloudflare.UnvalidatedIngressRule{
				Hostname:      "ingress.example.com",
				Path:          "/",
				Service:       "http://10.0.0.1:80",
				OriginRequest: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fromExposureToCloudflareIngress(tt.args.ctx, tt.args.exposure)
			if (err != nil) != tt.wantErr {
				t.Errorf("fromExposureToCloudflareIngress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("fromExposureToCloudflareIngress() got = %v, want %v", got, tt.want)
			}
		})
	}
}
