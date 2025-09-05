package cloudflarecontroller

import (
	"context"
	"reflect"
	"testing"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/cloudflare/cloudflare-go/v6"
	"github.com/cloudflare/cloudflare-go/v6/zero_trust"
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
		want    zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress
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
			want:    zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress{},
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
			want: zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress{
				Hostname: cloudflare.F("ingress.example.com"),
				Path:     cloudflare.F("/"),
				Service:  cloudflare.F("http://10.0.0.1:80"),
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
			want: zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress{
				Hostname: cloudflare.F("ingress.example.com"),
				Path:     cloudflare.F("/prefix"),
				Service:  cloudflare.F("http://10.0.0.1:80"),
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
			want: zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress{
				Hostname: cloudflare.F("ingress.example.com"),
				Path:     cloudflare.F("/prefix"),
				Service:  cloudflare.F("http://10.0.0.1:80"),
				OriginRequest: cloudflare.F(zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngressOriginRequest{
					HTTPHostHeader: cloudflare.F("foo.internal"),
				}),
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
			want: zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress{
				Hostname: cloudflare.F("ingress.example.com"),
				Path:     cloudflare.F("/"),
				Service:  cloudflare.F("https://10.0.0.1:443"),
				OriginRequest: cloudflare.F(zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngressOriginRequest{
					NoTLSVerify:      cloudflare.F(true),
					OriginServerName: cloudflare.F("bar.internal"),
				}),
			},
			wantErr: false,
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
			want: zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress{
				Hostname: cloudflare.F("ingress.example.com"),
				Path:     cloudflare.F("/"),
				Service:  cloudflare.F("https://10.0.0.1:443"),
				OriginRequest: cloudflare.F(zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngressOriginRequest{
					NoTLSVerify:      cloudflare.F(true),
					HTTPHostHeader:   cloudflare.F("foo.internal"),
					OriginServerName: cloudflare.F("bar.internal"),
				}),
			},
			wantErr: false,
		},
		{
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
			want: zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress{
				Hostname: cloudflare.F("ingress.example.com"),
				Path:     cloudflare.F("/"),
				Service:  cloudflare.F("https://10.0.0.1:443"),
				OriginRequest: cloudflare.F(zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngressOriginRequest{
					NoTLSVerify: cloudflare.F(true),
				}),
			},
			wantErr: false,
		},
		{
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
			want: zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress{
				Hostname: cloudflare.F("ingress.example.com"),
				Path:     cloudflare.F("/"),
				Service:  cloudflare.F("https://10.0.0.1:443"),
				OriginRequest: cloudflare.F(zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngressOriginRequest{
					NoTLSVerify: cloudflare.F(true),
				}),
			},
			wantErr: false,
		},
		{
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
			want: zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress{
				Hostname: cloudflare.F("ingress.example.com"),
				Path:     cloudflare.F("/"),
				Service:  cloudflare.F("https://10.0.0.1:443"),
				OriginRequest: cloudflare.F(zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngressOriginRequest{
					NoTLSVerify: cloudflare.F(false),
				}),
			},
			wantErr: false,
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
