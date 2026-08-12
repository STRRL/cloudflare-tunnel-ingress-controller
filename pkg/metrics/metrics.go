// Package metrics holds the custom prometheus metrics of the controller.
// All metrics are registered into the controller-runtime registry, so they
// are served on the same /metrics endpoint as the built-in controller
// metrics (reconcile counts, workqueue depth, and so on).
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const namespace = "cloudflare_tunnel_ingress_controller"

var (
	// LastSuccessfulSyncTimestamp is the unix time of the last time the
	// controller pushed tunnel config and DNS records to Cloudflare with
	// no error. Alert on this value getting old to catch config drift or
	// a broken API token.
	LastSuccessfulSyncTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "last_successful_sync_timestamp_seconds",
		Help:      "Unix timestamp of the last successful sync to Cloudflare.",
	})

	// ManagedExposures is the number of active exposures (host and path
	// pairs) the controller currently manages.
	ManagedExposures = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "managed_exposures",
		Help:      "Number of active exposures managed by the controller.",
	})

	// CloudflareAPIErrors counts failed Cloudflare API calls by operation.
	CloudflareAPIErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "cloudflare_api_errors_total",
		Help:      "Total number of failed Cloudflare API calls.",
	}, []string{"operation"})

	// DNSRecordOperations counts DNS record changes applied to Cloudflare.
	DNSRecordOperations = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "dns_record_operations_total",
		Help:      "Total number of DNS record changes applied to Cloudflare.",
	}, []string{"operation", "record_type"})

	// ManagedAccessApplications is the number of Cloudflare Access
	// Applications the controller currently manages through
	// CloudflareAccess objects.
	ManagedAccessApplications = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "managed_access_applications",
		Help:      "Number of Cloudflare Access Applications managed by the controller.",
	})
)

func init() {
	crmetrics.Registry.MustRegister(
		LastSuccessfulSyncTimestamp,
		ManagedExposures,
		CloudflareAPIErrors,
		DNSRecordOperations,
		ManagedAccessApplications,
	)
}
