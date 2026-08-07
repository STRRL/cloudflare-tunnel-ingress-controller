// Prometheus alert rules for the controller and the managed cloudflared
// connectors. Compile with `jsonnet -S` to get plain YAML that can be
// loaded by Prometheus or pasted into a PrometheusRule object.
local rules = {
  groups: [
    {
      name: 'cloudflare-tunnel-ingress-controller',
      rules: [
        {
          alert: 'CloudflareTunnelSyncStale',
          expr: 'time() - max(cloudflare_tunnel_ingress_controller_last_successful_sync_timestamp_seconds) > 300',
          'for': '5m',
          labels: { severity: 'critical' },
          annotations: {
            summary: 'Controller has not synced to Cloudflare recently',
            description: 'The last successful sync to Cloudflare is older than 5 minutes. Ingress changes are not reaching the tunnel. Check the controller logs and the Cloudflare API token.',
          },
        },
        {
          alert: 'CloudflareTunnelSyncMissing',
          expr: 'absent(cloudflare_tunnel_ingress_controller_last_successful_sync_timestamp_seconds)',
          'for': '10m',
          labels: { severity: 'critical' },
          annotations: {
            summary: 'Controller sync metric is missing',
            description: 'The sync timestamp metric is absent. The controller is down or Prometheus is not scraping it.',
          },
        },
        {
          alert: 'CloudflareTunnelAPIErrors',
          expr: 'sum(increase(cloudflare_tunnel_ingress_controller_cloudflare_api_errors_total[15m])) > 0',
          'for': '15m',
          labels: { severity: 'warning' },
          annotations: {
            summary: 'Cloudflare API calls are failing',
            description: 'The controller keeps hitting Cloudflare API errors. Check the controller logs for the failing operation.',
          },
        },
      ],
    },
    {
      name: 'cloudflare-tunnel-cloudflared',
      rules: [
        {
          alert: 'CloudflareTunnelHigh5xxRate',
          expr: |||
            sum by (host) (rate(cloudflared_tunnel_host_requests_total{status=~"5.."}[5m]))
              / sum by (host) (rate(cloudflared_tunnel_host_requests_total[5m]))
              > 0.05
          |||,
          'for': '10m',
          labels: { severity: 'warning' },
          annotations: {
            summary: 'High 5xx rate for {{ $labels.host }}',
            description: 'More than 5% of requests to {{ $labels.host }} return a 5xx status. Check the origin service behind this hostname.',
          },
        },
        {
          alert: 'CloudflareTunnelProxyErrors',
          expr: 'sum by (host) (rate(cloudflared_tunnel_host_request_errors_total[5m])) > 0.1',
          'for': '10m',
          labels: { severity: 'warning' },
          annotations: {
            summary: 'cloudflared cannot reach the origin for {{ $labels.host }}',
            description: 'cloudflared keeps failing to proxy requests for {{ $labels.host }}. The origin service is likely unreachable from the connector pod.',
          },
        },
      ],
    },
  ],
};

std.manifestYamlDoc(rules, quote_keys=false)
