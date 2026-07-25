package controller

import (
	"reflect"
	"testing"
	"time"

	"k8s.io/utils/ptr"
)

func Test_parseOriginRequestSettings(t *testing.T) {
	tests := []struct {
		name        string
		scheme      string
		annotations map[string]string
		want        originRequestSettings
		wantErr     bool
	}{
		{
			name:        "no annotations leaves everything nil",
			annotations: map[string]string{},
			want:        originRequestSettings{},
		},
		{
			name:   "all fields set",
			scheme: "https",
			annotations: map[string]string{
				AnnotationConnectTimeout:         "30s",
				AnnotationTLSTimeout:             "10s",
				AnnotationTCPKeepAlive:           "1m",
				AnnotationNoHappyEyeballs:        "true",
				AnnotationKeepAliveConnections:   "100",
				AnnotationKeepAliveTimeout:       "90s",
				AnnotationNoTLSVerify:            "false",
				AnnotationDisableChunkedEncoding: "true",
				AnnotationHTTP2Origin:            "true",
			},
			want: originRequestSettings{
				ConnectTimeout:         ptr.To(30 * time.Second),
				TLSTimeout:             ptr.To(10 * time.Second),
				TCPKeepAlive:           ptr.To(time.Minute),
				NoHappyEyeballs:        ptr.To(true),
				KeepAliveConnections:   ptr.To(100),
				KeepAliveTimeout:       ptr.To(90 * time.Second),
				NoTLSVerify:            ptr.To(false),
				DisableChunkedEncoding: ptr.To(true),
				HTTP2Origin:            ptr.To(true),
			},
		},
		{
			name: "invalid bool value",
			annotations: map[string]string{
				AnnotationHTTP2Origin: "yes",
			},
			wantErr: true,
		},
		{
			name: "invalid duration value",
			annotations: map[string]string{
				AnnotationConnectTimeout: "30",
			},
			wantErr: true,
		},
		{
			name: "sub second duration rejected",
			annotations: map[string]string{
				AnnotationConnectTimeout: "1500ms",
			},
			wantErr: true,
		},
		{
			name: "negative duration rejected",
			annotations: map[string]string{
				AnnotationTLSTimeout: "-10s",
			},
			wantErr: true,
		},
		{
			name: "invalid integer value",
			annotations: map[string]string{
				AnnotationKeepAliveConnections: "many",
			},
			wantErr: true,
		},
		{
			name: "zero keepalive connections disables the idle pool",
			annotations: map[string]string{
				AnnotationKeepAliveConnections: "0",
			},
			want: originRequestSettings{
				KeepAliveConnections: ptr.To(0),
			},
		},
		{
			name: "negative integer rejected",
			annotations: map[string]string{
				AnnotationKeepAliveConnections: "-1",
			},
			wantErr: true,
		},
		{
			name: "http2-origin requires https backend",
			annotations: map[string]string{
				AnnotationHTTP2Origin: "true",
			},
			wantErr: true,
		},
		{
			name:   "http2-origin allowed on https backend",
			scheme: "https",
			annotations: map[string]string{
				AnnotationHTTP2Origin: "true",
			},
			want: originRequestSettings{
				HTTP2Origin: ptr.To(true),
			},
		},
		{
			name: "explicit http2-origin false allowed on http backend",
			annotations: map[string]string{
				AnnotationHTTP2Origin: "false",
			},
			want: originRequestSettings{
				HTTP2Origin: ptr.To(false),
			},
		},
		{
			name: "no-tls-verify conflicts with proxy-ssl-verify",
			annotations: map[string]string{
				AnnotationNoTLSVerify:    "true",
				AnnotationProxySSLVerify: "off",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := tt.scheme
			if scheme == "" {
				scheme = "http"
			}
			got, err := parseOriginRequestSettings(tt.annotations, scheme)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseOriginRequestSettings() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseOriginRequestSettings() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
