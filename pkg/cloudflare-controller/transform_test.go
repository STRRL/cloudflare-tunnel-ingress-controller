package cloudflarecontroller

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/cloudflare/cloudflare-go"
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
					Hostname:      "ingress.example.com",
					ServiceTarget: "https://10.0.0.1:443",
					PathPrefix:    "/",
					IsDeleted:     false,
					NoTLSVerify:   boolPointer(true),
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
					Hostname:      "ingress.example.com",
					ServiceTarget: "https://10.0.0.1:443",
					PathPrefix:    "/",
					IsDeleted:     false,
					NoTLSVerify:   boolPointer(false),
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
			name: "origin server name",
			args: args{
				ctx: context.Background(),
				exposure: exposure.Exposure{
					Hostname:         "ingress.example.com",
					ServiceTarget:    "https://10.0.0.1:443",
					PathPrefix:       "/",
					IsDeleted:        false,
					OriginServerName: stringPointer("example.com"),
				},
			},
			want: &cloudflare.UnvalidatedIngressRule{
				Hostname: "ingress.example.com",
				Path:     "/",
				Service:  "https://10.0.0.1:443",
				OriginRequest: &cloudflare.OriginRequestConfig{
					OriginServerName: stringPointer("example.com"),
					NoTLSVerify:      boolPointer(true),
				},
			},
		}, {
			name: "origin CA pool",
			args: args{
				ctx: context.Background(),
				exposure: exposure.Exposure{
					Hostname:      "ingress.example.com",
					ServiceTarget: "http://10.0.0.1:443",
					PathPrefix:    "/",
					IsDeleted:     false,
					CAPool:        stringPointer("/path/to/my/certs.crt"),
				},
			},
			want: &cloudflare.UnvalidatedIngressRule{
				Hostname: "ingress.example.com",
				Path:     "/",
				Service:  "http://10.0.0.1:443",
				OriginRequest: &cloudflare.OriginRequestConfig{
					CAPool: stringPointer("/path/to/my/certs.crt"),
				},
			},
		}, {
			name: "valid origin TLS timeout",
			args: args{
				ctx: context.Background(),
				exposure: exposure.Exposure{
					Hostname:      "ingress.example.com",
					ServiceTarget: "http://10.0.0.1:443",
					PathPrefix:    "/",
					IsDeleted:     false,
					TLSTimeout:    parseDuration("30s"),
				},
			},
			want: &cloudflare.UnvalidatedIngressRule{
				Hostname: "ingress.example.com",
				Path:     "/",
				Service:  "http://10.0.0.1:443",
				OriginRequest: &cloudflare.OriginRequestConfig{
					TLSTimeout: &cloudflare.TunnelDuration{
						Duration: *parseDuration("30s"),
					},
				},
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

func parseDuration(value string) *time.Duration {
	duration, err := time.ParseDuration(value)
	if err != nil {
		panic(err)
	}
	return &duration
}

func stringPointer(s string) *string {
	return &s
}
