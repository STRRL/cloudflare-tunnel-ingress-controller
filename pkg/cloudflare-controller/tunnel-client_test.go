package cloudflarecontroller

import (
	"slices"
	"testing"

	"github.com/cloudflare/cloudflare-go"
)

func Test_sortIngressRules(t *testing.T) {
	tests := []struct {
		name      string
		input     []cloudflare.UnvalidatedIngressRule
		wantOrder []string // expected hostname order after sort
	}{
		{
			name: "wildcard sorts after explicit hostname",
			input: []cloudflare.UnvalidatedIngressRule{
				{Hostname: "*.example.com", Path: "/"},
				{Hostname: "app.example.com", Path: "/"},
			},
			wantOrder: []string{"app.example.com", "*.example.com"},
		},
		{
			name: "multiple explicit hostnames before wildcard",
			input: []cloudflare.UnvalidatedIngressRule{
				{Hostname: "*.example.com", Path: "/"},
				{Hostname: "app.example.com", Path: "/"},
				{Hostname: "api.example.com", Path: "/"},
			},
			wantOrder: []string{"api.example.com", "app.example.com", "*.example.com"},
		},
		{
			name: "non-wildcard only sorts alphabetically",
			input: []cloudflare.UnvalidatedIngressRule{
				{Hostname: "z.example.com", Path: "/"},
				{Hostname: "a.example.com", Path: "/"},
			},
			wantOrder: []string{"a.example.com", "z.example.com"},
		},
		{
			name: "path length descending for same hostname",
			input: []cloudflare.UnvalidatedIngressRule{
				{Hostname: "app.example.com", Path: "/"},
				{Hostname: "app.example.com", Path: "/longer/path"},
			},
			wantOrder: []string{"app.example.com", "app.example.com"},
		},
		{
			name: "single character subdomain sorts before wildcard",
			input: []cloudflare.UnvalidatedIngressRule{
				{Hostname: "*.example.com", Path: "/"},
				{Hostname: "x.example.com", Path: "/"},
			},
			wantOrder: []string{"x.example.com", "*.example.com"},
		},
		{
			name: "wildcard and explicit with different domains",
			input: []cloudflare.UnvalidatedIngressRule{
				{Hostname: "*.b.example.com", Path: "/"},
				{Hostname: "*.a.example.com", Path: "/"},
				{Hostname: "app.b.example.com", Path: "/"},
				{Hostname: "app.a.example.com", Path: "/"},
			},
			wantOrder: []string{"app.a.example.com", "app.b.example.com", "*.a.example.com", "*.b.example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := make([]cloudflare.UnvalidatedIngressRule, len(tt.input))
			copy(rules, tt.input)

			slices.SortFunc(rules, sortIngressRules)

			for i, rule := range rules {
				if rule.Hostname != tt.wantOrder[i] {
					t.Errorf("position %d: got hostname %q, want %q", i, rule.Hostname, tt.wantOrder[i])
				}
			}
		})
	}
}
