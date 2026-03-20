package cloudflarecontroller

import (
	"strings"
	"testing"

	"github.com/go-logr/logr"
)

func TestRenderDNSComment(t *testing.T) {
	tests := []struct {
		name              string
		templateStr       string
		hostname          string
		tunnelName        string
		tunnelId          string
		wantContains      string
		wantEmpty         bool
		wantLengthOver100 bool
	}{
		{
			name:        "empty template disables comments",
			templateStr: "",
			hostname:    "app.example.com",
			wantEmpty:   true,
		},
		{
			name:         "default template renders correctly",
			templateStr:  "managed by cloudflare-tunnel-ingress-controller, tunnel [{{.TunnelName}}]",
			hostname:     "app.example.com",
			tunnelName:   "my-tunnel",
			tunnelId:     "abc-123",
			wantContains: "managed by cloudflare-tunnel-ingress-controller, tunnel [my-tunnel]",
		},
		{
			name:         "template with all variables",
			templateStr:  "tunnel={{.TunnelName}} id={{.TunnelId}} host={{.Hostname}}",
			hostname:     "app.example.com",
			tunnelName:   "my-tunnel",
			tunnelId:     "abc-123",
			wantContains: "tunnel=my-tunnel id=abc-123 host=app.example.com",
		},
		{
			name:         "template with only hostname",
			templateStr:  "record for {{.Hostname}}",
			hostname:     "sub.domain.example.com",
			tunnelName:   "t",
			tunnelId:     "id",
			wantContains: "record for sub.domain.example.com",
		},
		{
			name:              "long comment exceeds 100 chars",
			templateStr:       "this is a very long comment template that will definitely exceed one hundred characters when rendered with tunnel={{.TunnelName}}",
			hostname:          "app.example.com",
			tunnelName:        "my-long-tunnel-name",
			tunnelId:          "abc-123",
			wantLengthOver100: true,
		},
		{
			name:        "invalid template syntax degrades gracefully",
			templateStr: "{{.InvalidSyntax",
			hostname:    "app.example.com",
			wantEmpty:   true,
		},
		{
			name:         "static template without variables",
			templateStr:  "managed by controller",
			hostname:     "app.example.com",
			tunnelName:   "t",
			tunnelId:     "id",
			wantContains: "managed by controller",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := NewTunnelClient(logr.Discard(), nil, "acc", tt.tunnelId, tt.tunnelName, tt.templateStr)
			got := tc.renderDNSComment(tt.hostname)

			if tt.wantEmpty && got != "" {
				t.Errorf("expected empty comment, got %q", got)
			}
			if tt.wantContains != "" && got != tt.wantContains {
				t.Errorf("expected %q, got %q", tt.wantContains, got)
			}
			if tt.wantLengthOver100 && len(got) <= 100 {
				t.Errorf("expected comment length > 100, got %d: %q", len(got), got)
			}
			if !tt.wantEmpty && tt.wantContains != "" && strings.TrimSpace(got) == "" {
				t.Errorf("expected non-empty comment, got empty")
			}
		})
	}
}
