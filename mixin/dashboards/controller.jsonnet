// Dashboard for the controller itself: sync health, managed exposures,
// Cloudflare API errors, DNS record changes, and reconcile activity.
local g = import '../lib/grafana.libsonnet';

local panels = [
  g.stat(
    'Managed Exposures',
    [g.target('max(cloudflare_tunnel_ingress_controller_managed_exposures)', 'exposures')],
    { x: 0, y: 0, w: 6, h: 5 },
  ),
  g.stat(
    'Time Since Last Successful Sync',
    [g.target('time() - max(cloudflare_tunnel_ingress_controller_last_successful_sync_timestamp_seconds)', 'age')],
    { x: 6, y: 0, w: 6, h: 5 },
    unit='s',
  ),
  g.stat(
    'Cloudflare API Errors (1h)',
    [g.target('sum(increase(cloudflare_tunnel_ingress_controller_cloudflare_api_errors_total[1h])) or vector(0)', 'errors')],
    { x: 12, y: 0, w: 6, h: 5 },
    thresholds={
      mode: 'absolute',
      steps: [
        { color: 'green', value: null },
        { color: 'red', value: 1 },
      ],
    },
  ),
  g.stat(
    'Controller Pods Up',
    [g.target('count(up{job=~"$job"} == 1)', 'pods')],
    { x: 18, y: 0, w: 6, h: 5 },
  ),

  g.timeseries(
    'Cloudflare API Errors by Operation',
    [g.target('sum by (operation) (rate(cloudflare_tunnel_ingress_controller_cloudflare_api_errors_total[$__rate_interval]))', '{{operation}}')],
    { x: 0, y: 5, w: 12, h: 8 },
    unit='ops',
  ),
  g.timeseries(
    'DNS Record Operations',
    [g.target('sum by (operation, record_type) (rate(cloudflare_tunnel_ingress_controller_dns_record_operations_total[$__rate_interval]))', '{{operation}} {{record_type}}')],
    { x: 12, y: 5, w: 12, h: 8 },
    unit='ops',
  ),

  g.timeseries(
    'Reconcile Rate by Result',
    [g.target('sum by (result) (rate(controller_runtime_reconcile_total{job=~"$job"}[$__rate_interval]))', '{{result}}')],
    { x: 0, y: 13, w: 12, h: 8 },
    unit='ops',
  ),
  g.timeseries(
    'Reconcile Duration (p50 / p99)',
    [
      g.target('histogram_quantile(0.50, sum by (le) (rate(controller_runtime_reconcile_time_seconds_bucket{job=~"$job"}[$__rate_interval])))', 'p50'),
      g.target('histogram_quantile(0.99, sum by (le) (rate(controller_runtime_reconcile_time_seconds_bucket{job=~"$job"}[$__rate_interval])))', 'p99'),
    ],
    { x: 12, y: 13, w: 12, h: 8 },
    unit='s',
  ),

  g.timeseries(
    'Workqueue Depth',
    [g.target('sum by (name) (workqueue_depth{job=~"$job"})', '{{name}}')],
    { x: 0, y: 21, w: 12, h: 8 },
  ),
  g.timeseries(
    'Kubernetes API Requests by Status',
    [g.target('sum by (code) (rate(rest_client_requests_total{job=~"$job"}[$__rate_interval]))', '{{code}}')],
    { x: 12, y: 21, w: 12, h: 8 },
    unit='reqps',
  ),
];

g.dashboard(
  'Cloudflare Tunnel Ingress Controller',
  'cf-tunnel-ingress-controller',
  panels,
  variables=[
    g.queryVariable('job', 'label_values(controller_runtime_reconcile_total, job)', multi=true),
  ],
)
