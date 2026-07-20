package cloudflarecontroller

import (
	"slices"
	"strings"
	"testing"

	"github.com/cloudflare/cloudflare-go"
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

func Test_sortIngressRules(t *testing.T) {
	tests := []struct {
		name      string
		input     []cloudflare.UnvalidatedIngressRule
		wantOrder []cloudflare.UnvalidatedIngressRule
	}{
		{
			name: "wildcard sorts after explicit hostname",
			input: []cloudflare.UnvalidatedIngressRule{
				{Hostname: "*.example.com", Path: "/"},
				{Hostname: "app.example.com", Path: "/"},
			},
			wantOrder: []cloudflare.UnvalidatedIngressRule{
				{Hostname: "app.example.com", Path: "/"},
				{Hostname: "*.example.com", Path: "/"},
			},
		},
		{
			name: "multiple explicit hostnames before wildcard",
			input: []cloudflare.UnvalidatedIngressRule{
				{Hostname: "*.example.com", Path: "/"},
				{Hostname: "app.example.com", Path: "/"},
				{Hostname: "api.example.com", Path: "/"},
			},
			wantOrder: []cloudflare.UnvalidatedIngressRule{
				{Hostname: "api.example.com", Path: "/"},
				{Hostname: "app.example.com", Path: "/"},
				{Hostname: "*.example.com", Path: "/"},
			},
		},
		{
			name: "non-wildcard only sorts alphabetically",
			input: []cloudflare.UnvalidatedIngressRule{
				{Hostname: "z.example.com", Path: "/"},
				{Hostname: "a.example.com", Path: "/"},
			},
			wantOrder: []cloudflare.UnvalidatedIngressRule{
				{Hostname: "a.example.com", Path: "/"},
				{Hostname: "z.example.com", Path: "/"},
			},
		},
		{
			name: "path length descending for same hostname",
			input: []cloudflare.UnvalidatedIngressRule{
				{Hostname: "app.example.com", Path: "/"},
				{Hostname: "app.example.com", Path: "/longer/path"},
			},
			wantOrder: []cloudflare.UnvalidatedIngressRule{
				{Hostname: "app.example.com", Path: "/longer/path"},
				{Hostname: "app.example.com", Path: "/"},
			},
		},
		{
			name: "equal length paths order lexically",
			input: []cloudflare.UnvalidatedIngressRule{
				{Hostname: "app.example.com", Path: "/foo"},
				{Hostname: "app.example.com", Path: "/api"},
			},
			wantOrder: []cloudflare.UnvalidatedIngressRule{
				{Hostname: "app.example.com", Path: "/api"},
				{Hostname: "app.example.com", Path: "/foo"},
			},
		},
		{
			name: "single character subdomain sorts before wildcard",
			input: []cloudflare.UnvalidatedIngressRule{
				{Hostname: "*.example.com", Path: "/"},
				{Hostname: "x.example.com", Path: "/"},
			},
			wantOrder: []cloudflare.UnvalidatedIngressRule{
				{Hostname: "x.example.com", Path: "/"},
				{Hostname: "*.example.com", Path: "/"},
			},
		},
		{
			name: "apex domain sorts before its wildcard",
			input: []cloudflare.UnvalidatedIngressRule{
				{Hostname: "*.example.com", Path: "/"},
				{Hostname: "example.com", Path: "/"},
			},
			wantOrder: []cloudflare.UnvalidatedIngressRule{
				{Hostname: "example.com", Path: "/"},
				{Hostname: "*.example.com", Path: "/"},
			},
		},
		{
			name: "more specific wildcard sorts before broader wildcard",
			input: []cloudflare.UnvalidatedIngressRule{
				{Hostname: "*.example.com", Path: "/"},
				{Hostname: "*.internal.example.com", Path: "/"},
			},
			wantOrder: []cloudflare.UnvalidatedIngressRule{
				{Hostname: "*.internal.example.com", Path: "/"},
				{Hostname: "*.example.com", Path: "/"},
			},
		},
		{
			name: "wildcards with equal specificity sort alphabetically",
			input: []cloudflare.UnvalidatedIngressRule{
				{Hostname: "*.b.example.com", Path: "/"},
				{Hostname: "*.a.example.com", Path: "/"},
			},
			wantOrder: []cloudflare.UnvalidatedIngressRule{
				{Hostname: "*.a.example.com", Path: "/"},
				{Hostname: "*.b.example.com", Path: "/"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := make([]cloudflare.UnvalidatedIngressRule, len(tt.input))
			copy(rules, tt.input)

			slices.SortFunc(rules, sortIngressRules)

			for i, rule := range rules {
				if rule.Hostname != tt.wantOrder[i].Hostname || rule.Path != tt.wantOrder[i].Path {
					t.Errorf("position %d: got %s%s, want %s%s", i, rule.Hostname, rule.Path, tt.wantOrder[i].Hostname, tt.wantOrder[i].Path)
				}
			}
		})
	}
}
