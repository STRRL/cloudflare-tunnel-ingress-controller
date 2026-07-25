package e2e

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/cucumber/godog"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/utils/ptr"
)

// the annotations the scenario applies, a representative subset proving the
// annotation to tunnel configuration wiring; the remaining fields share the
// same code path and are covered by unit tests
var originRequestTestAnnotations = map[string]string{
	"cloudflare-tunnel-ingress-controller.strrl.dev/connect-timeout":          "31s",
	"cloudflare-tunnel-ingress-controller.strrl.dev/keepalive-connections":    "7",
	"cloudflare-tunnel-ingress-controller.strrl.dev/keepalive-timeout":        "77s",
	"cloudflare-tunnel-ingress-controller.strrl.dev/disable-chunked-encoding": "true",
}

func registerOriginRequestSteps(ctx *godog.ScenarioContext) {
	ctx.Step(`^an ingress with origin request annotations exposes "([^"]*)" at a generated hostname$`, anIngressWithOriginRequestAnnotationsExposes)
	ctx.Step(`^the tunnel ingress rule eventually carries the expected origin request settings$`, theTunnelRuleCarriesOriginRequestSettings)
}

func anIngressWithOriginRequestAnnotationsExposes(ctx context.Context, serviceName string) error {
	w := worldFromContext(ctx)

	hostname, err := buildTestHostname("cf-origin", baseDomain)
	if err != nil {
		return err
	}
	w.hostname = hostname
	_, _ = fmt.Fprintf(logOut, "using origin request hostname %s\n", hostname)

	return createScenarioIngress(w, namespacedName{namespace: "default", name: "origin-request-via-cloudflare"}, originRequestTestAnnotations, []networkingv1.IngressRule{
		serviceIngressRule(hostname, serviceName, 80),
	})
}

func theTunnelRuleCarriesOriginRequestSettings(ctx context.Context) error {
	w := worldFromContext(ctx)

	api, err := newCloudflareAPI()
	if err != nil {
		return err
	}
	rc := cloudflare.ResourceIdentifier(os.Getenv("CLOUDFLARE_ACCOUNT_ID"))

	// tunnel discovery lives inside the retry loop so transient API failures
	// do not abort the scenario
	return waitFor("tunnel rule carries origin request settings", 5*time.Minute, 10*time.Second, func() error {
		tunnels, _, err := api.ListTunnels(context.Background(), rc, cloudflare.TunnelListParams{
			Name:      os.Getenv("CLOUDFLARE_TUNNEL_NAME"),
			IsDeleted: ptr.To(false),
		})
		if err != nil {
			return fmt.Errorf("list tunnels: %w", err)
		}
		if len(tunnels) == 0 {
			return fmt.Errorf("tunnel %s not found", os.Getenv("CLOUDFLARE_TUNNEL_NAME"))
		}
		tunnelID := tunnels[0].ID

		configuration, err := api.GetTunnelConfiguration(context.Background(), rc, tunnelID)
		if err != nil {
			return fmt.Errorf("get tunnel configuration: %w", err)
		}

		for _, rule := range configuration.Config.Ingress {
			if rule.Hostname != w.hostname {
				continue
			}
			origin := rule.OriginRequest
			if origin == nil {
				return fmt.Errorf("rule for %s has no originRequest yet", w.hostname)
			}
			if origin.ConnectTimeout == nil || origin.ConnectTimeout.Duration != 31*time.Second {
				return fmt.Errorf("rule for %s has connectTimeout %v, want 31s", w.hostname, origin.ConnectTimeout)
			}
			if origin.KeepAliveConnections == nil || *origin.KeepAliveConnections != 7 {
				return fmt.Errorf("rule for %s has keepAliveConnections %v, want 7", w.hostname, origin.KeepAliveConnections)
			}
			if origin.KeepAliveTimeout == nil || origin.KeepAliveTimeout.Duration != 77*time.Second {
				return fmt.Errorf("rule for %s has keepAliveTimeout %v, want 77s", w.hostname, origin.KeepAliveTimeout)
			}
			if origin.DisableChunkedEncoding == nil || !*origin.DisableChunkedEncoding {
				return fmt.Errorf("rule for %s has disableChunkedEncoding %v, want true", w.hostname, origin.DisableChunkedEncoding)
			}
			return nil
		}
		return fmt.Errorf("no tunnel rule for hostname %s yet", w.hostname)
	})
}
