package controller

import (
	"cmp"
	"context"
	"reflect"
	"slices"
	"sync"

	cloudflarecontroller "github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/cloudflare-controller"
	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/pkg/errors"
)

// TunnelSync pushes the exposures of Ingress and Gateway resources to the
// one shared tunnel. PutExposures replaces the whole tunnel configuration and
// removes DNS records it does not see, so both sides must always be pushed
// together.
type TunnelSync struct {
	mutex        sync.Mutex
	tunnelClient cloudflarecontroller.TunnelClientInterface

	// listIngressExposures returns the current Ingress exposures without
	// recording events, used when the Gateway side triggers a sync.
	listIngressExposures func(ctx context.Context) ([]exposure.Exposure, error)

	gatewayEnabled   bool
	gatewayKnown     bool
	gatewayExposures []exposure.Exposure

	hasSynced  bool
	lastSynced []exposure.Exposure
}

func NewTunnelSync(tunnelClient cloudflarecontroller.TunnelClientInterface, gatewayEnabled bool) *TunnelSync {
	return &TunnelSync{tunnelClient: tunnelClient, gatewayEnabled: gatewayEnabled}
}

// SyncIngress pushes the Ingress exposures together with the last known
// Gateway exposures. It always calls Cloudflare, like the Ingress controller
// always did, so manual changes in Cloudflare get repaired.
func (s *TunnelSync) SyncIngress(ctx context.Context, ingressExposures []exposure.Exposure) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// before the first Gateway reconcile the Gateway exposures are unknown,
	// pushing now would delete the DNS records of every Gateway
	if s.gatewayEnabled && !s.gatewayKnown {
		return errors.New("gateway exposures are not computed yet, retry later")
	}
	return s.push(ctx, append(slices.Clone(ingressExposures), s.gatewayExposures...))
}

// SyncGateway stores the Gateway exposures and pushes them together with the
// Ingress exposures, but only when the combined set changed since the last
// successful push. Route changes never reach this point with a different
// set, so they cost no Cloudflare API calls.
func (s *TunnelSync) SyncGateway(ctx context.Context, gatewayExposures []exposure.Exposure) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.gatewayExposures = slices.Clone(gatewayExposures)
	s.gatewayKnown = true

	var ingressExposures []exposure.Exposure
	if s.listIngressExposures != nil {
		var err error
		ingressExposures, err = s.listIngressExposures(ctx)
		if err != nil {
			return errors.Wrap(err, "list ingress exposures")
		}
	}
	all := append(ingressExposures, s.gatewayExposures...)
	if s.hasSynced && reflect.DeepEqual(sortedExposures(all), sortedExposures(s.lastSynced)) {
		return nil
	}
	return s.push(ctx, all)
}

func (s *TunnelSync) push(ctx context.Context, all []exposure.Exposure) error {
	err := s.tunnelClient.PutExposures(ctx, all)
	if err != nil {
		s.hasSynced = false
		return err
	}
	s.hasSynced = true
	s.lastSynced = all
	return nil
}

// sortedExposures returns a sorted copy, list order from the cache is random.
func sortedExposures(items []exposure.Exposure) []exposure.Exposure {
	result := slices.Clone(items)
	slices.SortFunc(result, func(a, b exposure.Exposure) int {
		return cmp.Or(
			cmp.Compare(a.Hostname, b.Hostname),
			cmp.Compare(a.PathPrefix, b.PathPrefix),
			cmp.Compare(a.ServiceTarget, b.ServiceTarget),
		)
	})
	return result
}
