package conformance

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

// newAuthoritativeDialer returns a dial function that resolves hostnames
// below baseDomain with the zone's authoritative nameservers. New Gateway
// records are visible there at once, while a local resolver may cache a
// negative answer from before the record existed. Other hostnames use the
// system resolver. The edge still receives the real proxied record, nothing
// bypasses Cloudflare.
//
// resolverOverride (host:port) replaces the authoritative nameserver, for
// networks that block outbound DNS to it.
func newAuthoritativeDialer(baseDomain string, resolverOverride string) (func(ctx context.Context, network string, address string) (net.Conn, error), error) {
	nameserver := resolverOverride
	if nameserver == "" {
		var err error
		nameserver, err = findNameserver(baseDomain)
		if err != nil {
			return nil, err
		}
	}

	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network string, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, network, nameserver)
		},
	}
	dialer := &net.Dialer{Timeout: 30 * time.Second}
	suffix := "." + strings.ToLower(baseDomain)

	return func(ctx context.Context, network string, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if !strings.HasSuffix(strings.ToLower(host), suffix) {
			return dialer.DialContext(ctx, network, address)
		}
		ips, err := resolver.LookupIP(ctx, "ip4", host)
		if err != nil {
			return nil, fmt.Errorf("resolve %s with the authoritative nameserver: %w", host, err)
		}
		var lastErr error
		for _, ip := range ips {
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		return nil, lastErr
	}, nil
}

// findNameserver returns host:53 of an authoritative nameserver of the zone
// holding domain, walking up the labels until NS records exist.
func findNameserver(domain string) (string, error) {
	labels := strings.Split(strings.TrimSuffix(domain, "."), ".")
	for i := 0; i < len(labels)-1; i++ {
		candidate := strings.Join(labels[i:], ".")
		records, err := net.LookupNS(candidate)
		if err != nil || len(records) == 0 {
			continue
		}
		return net.JoinHostPort(strings.TrimSuffix(records[0].Host, "."), "53"), nil
	}
	return "", fmt.Errorf("no NS records found for the base domain or its parents")
}
