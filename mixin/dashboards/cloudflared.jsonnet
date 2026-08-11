// Dashboard for the managed cloudflared connectors: per hostname traffic
// from the patched image (ghcr.io/strrl/cloudflared) plus tunnel wide
// request and error rates.
local g = import '../lib/grafana.libsonnet';

local hostFilter = 'host=~"$host"';

local panels = [
  g.stat(
    'Request Rate',
    [g.target('sum(rate(cloudflared_tunnel_host_requests_total{%s}[$__rate_interval]))' % hostFilter, 'req/s')],
    { x: 0, y: 0, w: 6, h: 5 },
    unit='reqps',
  ),
  g.stat(
    'Error Rate (5xx)',
    [g.target('sum(rate(cloudflared_tunnel_host_requests_total{%s, status=~"5.."}[$__rate_interval])) or vector(0)' % hostFilter, '5xx/s')],
    { x: 6, y: 0, w: 6, h: 5 },
    unit='reqps',
    thresholds={
      mode: 'absolute',
      steps: [
        { color: 'green', value: null },
        { color: 'red', value: 0.1 },
      ],
    },
  ),
  g.stat(
    'p99 Request Duration',
    [g.target('histogram_quantile(0.99, sum by (le) (rate(cloudflared_tunnel_host_request_duration_seconds_bucket{%s}[$__rate_interval])))' % hostFilter, 'p99')],
    { x: 12, y: 0, w: 6, h: 5 },
    unit='s',
  ),
  g.stat(
    'Connectors Up',
    [g.target('count(up{job=~".*cloudflared.*"} == 1) or vector(0)', 'pods')],
    { x: 18, y: 0, w: 6, h: 5 },
  ),

  g.timeseries(
    'Request Rate by Host',
    [g.target('sum by (host) (rate(cloudflared_tunnel_host_requests_total{%s}[$__rate_interval]))' % hostFilter, '{{host}}')],
    { x: 0, y: 5, w: 12, h: 8 },
    unit='reqps',
  ),
  g.timeseries(
    'Responses by Status Class',
    [g.target('sum by (status) (rate(cloudflared_tunnel_host_requests_total{%s}[$__rate_interval]))' % hostFilter, '{{status}}')],
    { x: 12, y: 5, w: 12, h: 8 },
    unit='reqps',
  ),

  g.timeseries(
    'Request Duration Percentiles',
    [
      g.target('histogram_quantile(0.50, sum by (le) (rate(cloudflared_tunnel_host_request_duration_seconds_bucket{%s}[$__rate_interval])))' % hostFilter, 'p50'),
      g.target('histogram_quantile(0.90, sum by (le) (rate(cloudflared_tunnel_host_request_duration_seconds_bucket{%s}[$__rate_interval])))' % hostFilter, 'p90'),
      g.target('histogram_quantile(0.95, sum by (le) (rate(cloudflared_tunnel_host_request_duration_seconds_bucket{%s}[$__rate_interval])))' % hostFilter, 'p95'),
      g.target('histogram_quantile(0.99, sum by (le) (rate(cloudflared_tunnel_host_request_duration_seconds_bucket{%s}[$__rate_interval])))' % hostFilter, 'p99'),
    ],
    { x: 0, y: 13, w: 12, h: 8 },
    unit='s',
  ),
  g.timeseries(
    'Origin Latency Breakdown (p99)',
    [
      g.target('histogram_quantile(0.99, sum by (le) (rate(cloudflared_tunnel_host_connect_duration_seconds_bucket{%s}[$__rate_interval])))' % hostFilter, 'connect'),
      g.target('histogram_quantile(0.99, sum by (le) (rate(cloudflared_tunnel_host_header_duration_seconds_bucket{%s}[$__rate_interval])))' % hostFilter, 'first header'),
      g.target('histogram_quantile(0.99, sum by (le) (rate(cloudflared_tunnel_host_response_duration_seconds_bucket{%s}[$__rate_interval])))' % hostFilter, 'full response'),
    ],
    { x: 12, y: 13, w: 12, h: 8 },
    unit='s',
  ),

  g.heatmap(
    'Request Duration Heatmap',
    'sum by (le) (increase(cloudflared_tunnel_host_request_duration_seconds_bucket{%s}[$__rate_interval]))' % hostFilter,
    { x: 0, y: 21, w: 24, h: 8 },
  ),

  g.timeseries(
    'Proxy Errors by Host',
    [g.target('sum by (host) (rate(cloudflared_tunnel_host_request_errors_total{%s}[$__rate_interval]))' % hostFilter, '{{host}}')],
    { x: 0, y: 29, w: 12, h: 8 },
    unit='ops',
  ),
  g.timeseries(
    'Traffic Volume by Host',
    [
      g.target('sum by (host) (rate(cloudflared_tunnel_host_request_body_size_bytes_sum{%s}[$__rate_interval]))' % hostFilter, 'in {{host}}'),
      g.target('-sum by (host) (rate(cloudflared_tunnel_host_response_body_size_bytes_sum{%s}[$__rate_interval]))' % hostFilter, 'out {{host}}'),
    ],
    { x: 12, y: 29, w: 12, h: 8 },
    unit='Bps',
  ),

  g.timeseries(
    'Concurrent Requests per Tunnel',
    [g.target('sum(cloudflared_tunnel_concurrent_requests_per_tunnel)', 'concurrent')],
    { x: 0, y: 37, w: 12, h: 8 },
  ),
  g.timeseries(
    'Active TCP Sessions',
    [g.target('sum(cloudflared_tcp_active_sessions)', 'sessions')],
    { x: 12, y: 37, w: 12, h: 8 },
  ),
];

g.dashboard(
  'Cloudflare Tunnel Traffic by Hostname',
  'cf-tunnel-cloudflared-traffic',
  panels,
  variables=[
    g.queryVariable('host', 'label_values(cloudflared_tunnel_host_requests_total, host)', multi=true),
  ],
)
