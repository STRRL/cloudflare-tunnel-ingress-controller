package cloudflarecontroller

import "testing"

func TestDomain_IsSubDomainOf(t *testing.T) {
	tests := []struct {
		name     string
		domain   Domain
		target   Domain
		expected bool
	}{
		{
			name:     "Valid subdomain",
			domain:   Domain{Name: "sub.example.com"},
			target:   Domain{Name: "example.com"},
			expected: true,
		},
		{
			name:     "Same domain",
			domain:   Domain{Name: "example.com"},
			target:   Domain{Name: "example.com"},
			expected: false,
		},
		{
			name:     "Different TLD",
			domain:   Domain{Name: "example.com"},
			target:   Domain{Name: "example.org"},
			expected: false,
		},
		{
			name:     "Subdomain with multiple levels",
			domain:   Domain{Name: "a.b.c.example.com"},
			target:   Domain{Name: "example.com"},
			expected: true,
		},
		{
			name:     "Case insensitive",
			domain:   Domain{Name: "Sub.Example.Com"},
			target:   Domain{Name: "example.COM"},
			expected: true,
		},
		{
			name:     "Similar but not subdomain",
			domain:   Domain{Name: "site1.example.com"},
			target:   Domain{Name: "myexample.com"},
			expected: false,
		},
		{
			name:     "Subdomain with different prefix",
			domain:   Domain{Name: "blog.example.com"},
			target:   Domain{Name: "shop.example.com"},
			expected: false,
		},
		{
			name:     "Empty domain names",
			domain:   Domain{Name: ""},
			target:   Domain{Name: ""},
			expected: false,
		},
		{
			name:     "Domain with only TLD",
			domain:   Domain{Name: "com"},
			target:   Domain{Name: ""},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.domain.IsSubDomainOf(tt.target); got != tt.expected {
				t.Errorf("Domain.IsSubDomainOf() = %v, want %v", got, tt.expected)
			}
		})
	}
}
