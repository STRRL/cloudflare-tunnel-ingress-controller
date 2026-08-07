# Grafana Dashboards

Grafana dashboards for the controller and its managed cloudflared connectors,
written in jsonnet.

## Ready to import JSON

The compiled dashboards live in `dist/`:

- `dist/controller.json`: controller health. Sync freshness, managed
  exposures, Cloudflare API errors, DNS record changes, reconcile activity,
  and workqueue depth.
- `dist/cloudflared.json`: per hostname traffic. Request rate, status codes,
  latency percentiles, origin latency breakdown, proxy errors, and traffic
  volume for every exposed hostname.

Import them in Grafana with `Dashboards -> New -> Import`, then pick your
Prometheus data source.

The per hostname panels need the patched cloudflared image
(`ghcr.io/strrl/cloudflared`), which is the chart default. See the
[monitoring guide](https://tunnel.strrl.dev/how-to/monitoring/) for how to
scrape both metrics endpoints.

## Rebuild from source

The jsonnet sources are in `dashboards/` with a small helper library in
`lib/`. After changing them, rebuild the JSON files:

```bash
make dashboards
```

This requires the [jsonnet](https://jsonnet.org/) binary.
