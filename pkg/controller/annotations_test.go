package controller

import (
	"reflect"
	"testing"

	"github.com/cloudflare/cloudflare-go"
)

func Test_annotationsProperties(t *testing.T) {
	type args struct {
		annotations map[string]string
	}
	tests := []struct {
		name    string
		args    args
		want    *cloudflare.OriginRequestConfig
		wantErr bool
	}{
		{
			name: "empty annotations (no defaults)",
			args: args{
				annotations: map[string]string{},
			},
			want:    &cloudflare.OriginRequestConfig{},
			wantErr: false,
		},
		{
			name: "set bool values for annotations (on)",
			args: args{
				annotations: map[string]string{
					AnnotationHTTP20Origin:    "on",
					AnnotationChunkedEncoding: "on",
					AnnotationProxySSLVerify:  "on",
					AnnotationHappyEyeballs:   "on",
				},
			},
			want: &cloudflare.OriginRequestConfig{
				NoTLSVerify:            boolPointer(false),
				DisableChunkedEncoding: boolPointer(false),
				Http2Origin:            boolPointer(true),
				NoHappyEyeballs:        boolPointer(false),
			},
			wantErr: false,
		},
		{
			name: "set bool values for annotations (off)",
			args: args{
				annotations: map[string]string{
					AnnotationHTTP20Origin:    "off",
					AnnotationChunkedEncoding: "off",
					AnnotationProxySSLVerify:  "off",
					AnnotationHappyEyeballs:   "off",
				},
			},
			want: &cloudflare.OriginRequestConfig{
				NoTLSVerify:            boolPointer(true),
				DisableChunkedEncoding: boolPointer(true),
				Http2Origin:            boolPointer(false),
				NoHappyEyeballs:        boolPointer(true),
			},
			wantErr: false,
		},
		{
			name: "invalid value (AnnotationHTTP20Origin)",
			args: args{
				annotations: map[string]string{
					AnnotationHTTP20Origin: "false",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid value (ProxySSLVerifyEnabled)",
			args: args{
				annotations: map[string]string{
					AnnotationProxySSLVerify: "false",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid value (AnnotationChunkedEncoding)",
			args: args{
				annotations: map[string]string{
					AnnotationChunkedEncoding: "false",
				},
			},
			wantErr: true,
		},
		{
			name: "set duration values",
			args: args{
				annotations: map[string]string{
					AnnotationTLSTimeout:          "50",
					AnnotationConnectionTimeount:  "100",
					AnnotationTCPKeepAliveTimeout: "150",
				},
			},
			want: &cloudflare.OriginRequestConfig{
				TLSTimeout:       &cloudflare.TunnelDuration{Duration: intToDurationSeconds(50)},
				ConnectTimeout:   &cloudflare.TunnelDuration{Duration: intToDurationSeconds(100)},
				KeepAliveTimeout: &cloudflare.TunnelDuration{Duration: intToDurationSeconds(150)},
			},
			wantErr: false,
		},
		{
			name: "invalid value (AnnotationTLSTimeout)",
			args: args{
				annotations: map[string]string{
					AnnotationTLSTimeout: "50s",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid value (AnnotationConnectionTimeount)",
			args: args{
				annotations: map[string]string{
					AnnotationConnectionTimeount: "true",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid value (AnnotationTCPKeepAliveTimeout)",
			args: args{
				annotations: map[string]string{
					AnnotationTCPKeepAliveTimeout: "not-time",
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := annotationProperties(tt.args.annotations)
			if (err != nil) != tt.wantErr {
				t.Errorf("annotationProperties() error = %+v, wantErr %+v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("annotationProperties() got = %+v, want %+v", got, tt.want)
			}
		})
	}
}
