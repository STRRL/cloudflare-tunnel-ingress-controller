package cloudflarecontroller

import (
	"context"
	"reflect"
	"testing"

	"github.com/cloudflare/cloudflare-go"
	"github.com/oliverbaehler/cloudflare-tunnel-ingress-controller/pkg/exposure"
)

var (
	hostname = "ingress.example.com"
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
					Hostname:      hostname,
					ServiceTarget: "http://10.0.0.1:80",
					PathPrefix:    "/",
					IsDeleted:     false,
					OriginRequest: cloudflare.OriginRequestConfig{
						HTTPHostHeader: &hostname,
						NoTLSVerify:    boolPointer(true),
					},
				},
			},
			want: &cloudflare.UnvalidatedIngressRule{
				Hostname: hostname,
				Path:     "/",
				Service:  "http://10.0.0.1:80",
				OriginRequest: &cloudflare.OriginRequestConfig{
					HTTPHostHeader: &hostname,
					NoTLSVerify:    boolPointer(true),
				},
			},
			wantErr: false,
		},
		{
			name: "contains path",
			args: args{
				ctx: context.Background(),
				exposure: exposure.Exposure{
					Hostname:      hostname,
					ServiceTarget: "http://10.0.0.1:80",
					PathPrefix:    "/prefix",
					IsDeleted:     false,
					OriginRequest: cloudflare.OriginRequestConfig{
						HTTPHostHeader: &hostname,
					},
				},
			},
			want: &cloudflare.UnvalidatedIngressRule{
				Hostname: hostname,
				Path:     "/prefix",
				Service:  "http://10.0.0.1:80",
				OriginRequest: &cloudflare.OriginRequestConfig{
					HTTPHostHeader: &hostname,
				},
			},
			wantErr: false,
		}, {
			name: "https should enable no-tls-verify by default",
			args: args{
				ctx: context.Background(),
				exposure: exposure.Exposure{
					Hostname:      hostname,
					ServiceTarget: "https://10.0.0.1:443",
					PathPrefix:    "/",
					IsDeleted:     false,
					OriginRequest: cloudflare.OriginRequestConfig{
						NoTLSVerify: boolPointer(true),
					},
				},
			},
			want: &cloudflare.UnvalidatedIngressRule{
				Hostname: hostname,
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
					Hostname:      hostname,
					ServiceTarget: "https://10.0.0.1:443",
					PathPrefix:    "/",
					IsDeleted:     false,
					OriginRequest: cloudflare.OriginRequestConfig{
						Http2Origin: boolPointer(false),
					},
				},
			},
			want: &cloudflare.UnvalidatedIngressRule{
				Hostname: hostname,
				Path:     "/",
				Service:  "https://10.0.0.1:443",
				OriginRequest: &cloudflare.OriginRequestConfig{
					Http2Origin: boolPointer(false),
				},
			},
		}, {
			name: "https with no-tls-verify disabled",
			args: args{
				ctx: context.Background(),
				exposure: exposure.Exposure{
					Hostname:      hostname,
					ServiceTarget: "https://10.0.0.1:443",
					PathPrefix:    "/",
					IsDeleted:     false,
					OriginRequest: cloudflare.OriginRequestConfig{
						Http2Origin: boolPointer(true),
					},
				},
			},
			want: &cloudflare.UnvalidatedIngressRule{
				Hostname: hostname,
				Path:     "/",
				Service:  "https://10.0.0.1:443",
				OriginRequest: &cloudflare.OriginRequestConfig{
					Http2Origin: boolPointer(true),
				},
			},
		},
		{
			name: "http2-disabled",
			args: args{
				ctx: context.Background(),
				exposure: exposure.Exposure{
					Hostname:      hostname,
					ServiceTarget: "https://10.0.0.1:443",
					PathPrefix:    "/",
					IsDeleted:     false,
					OriginRequest: cloudflare.OriginRequestConfig{
						Http2Origin: boolPointer(false),
					},
				},
			},
			want: &cloudflare.UnvalidatedIngressRule{
				Hostname: hostname,
				Path:     "/",
				Service:  "https://10.0.0.1:443",
				OriginRequest: &cloudflare.OriginRequestConfig{
					Http2Origin: boolPointer(false),
				},
			},
		},
		{
			name: "http2-enabled",
			args: args{
				ctx: context.Background(),
				exposure: exposure.Exposure{
					Hostname:      hostname,
					ServiceTarget: "https://10.0.0.1:443",
					PathPrefix:    "/",
					IsDeleted:     false,
					OriginRequest: cloudflare.OriginRequestConfig{
						Http2Origin: boolPointer(true),
					},
				},
			},
			want: &cloudflare.UnvalidatedIngressRule{
				Hostname: hostname,
				Path:     "/",
				Service:  "https://10.0.0.1:443",
				OriginRequest: &cloudflare.OriginRequestConfig{
					Http2Origin: boolPointer(true),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fromExposureToCloudflareIngress(tt.args.ctx, tt.args.exposure)
			if (err != nil) != tt.wantErr {
				t.Errorf("fromExposureToCloudflareIngress() error = %+v, wantErr %+v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("fromExposureToCloudflareIngress() got = %+v, want %+v", *got.OriginRequest, *tt.want.OriginRequest)
			}
		})
	}
}
