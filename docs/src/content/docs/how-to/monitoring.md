---
title: Monitor the Controller and cloudflared
description: Scrape controller and cloudflared metrics and configure connector health probes.
---

The controller and its managed `cloudflared` connectors expose separate metrics endpoints.

See [Helm values](/reference/helm-values/) for chart defaults.

## Inspect controller metrics

The controller serves controller runtime metrics plus custom sync metrics over HTTP at `/metrics` on port `9090` (`metrics.port`). The chart always creates a metrics Service for this endpoint. When `serviceMonitor.create` is false, the Service carries `prometheus.io/scrape` annotations for annotation based discovery.

Forward the port from one controller pod:

```bash
kubectl port-forward deployment/cloudflare-tunnel-ingress-controller \
  -n cloudflare-tunnel-ingress-controller \
  9090:9090
```

Read the endpoint from another terminal:

```bash
curl http://127.0.0.1:9090/metrics
```

## Inspect cloudflared metrics

Every managed `cloudflared` process listens on `0.0.0.0:44483`. The chart always creates the `controlled-cloudflared-connector-headless` Service with a `metrics` port at `44483`.

Forward that Service and read one connector:

```bash
kubectl port-forward service/controlled-cloudflared-connector-headless \
  -n cloudflare-tunnel-ingress-controller \
  44483:44483
```

```bash
curl http://127.0.0.1:44483/metrics
```

When `serviceMonitor.create` is false, the Service carries `prometheus.io/scrape: "true"` and `prometheus.io/port: "44483"` annotations for annotation based discovery.

## Create the ServiceMonitors

Prometheus Operator must be installed before you enable these objects. One switch creates ServiceMonitors for both metrics endpoints: the controller and the managed cloudflared connectors. Add the following settings to the values file used by the existing release:

```yaml
serviceMonitor:
  create: true
  labels:
    release: kube-prometheus-stack
  interval: 30s
  scrapeTimeout: 10s
  metricRelabelings: []
  relabelings: []
  cloudflared:
    jobLabel: app.kubernetes.io/component
    honorLabels: false
    scheme: http
```

Adjust each field for your Prometheus installation:

1. `create` renders both ServiceMonitors.
2. `labels` adds metadata labels, commonly used by a Prometheus selector.
3. `interval` sets the scrape interval.
4. `scrapeTimeout` sets the timeout for one scrape.
5. `metricRelabelings` adds metric relabel rules after a scrape.
6. `relabelings` adds target relabel rules before a scrape.
7. `cloudflared.jobLabel` names the selected Service label used as the Prometheus job label for the connector target.
8. `cloudflared.honorLabels` controls whether labels from scraped connector metrics take precedence.
9. `cloudflared.scheme` sets the scrape scheme for the connector target.

The generated ServiceMonitors select only the chart Services in the Helm release namespace and scrape their named `metrics` ports at `/metrics`.

Run your normal Helm upgrade, then verify the object:

```bash
kubectl get servicemonitor \
  -n cloudflare-tunnel-ingress-controller
```

## Import the Grafana dashboards

The repository ships two ready to import Grafana dashboards in the
[`mixin/dist`](https://github.com/STRRL/cloudflare-tunnel-ingress-controller/tree/master/mixin/dist) directory:

1. `controller.json` shows controller health: sync freshness, managed exposures, Cloudflare API errors, DNS record changes, and reconcile activity.
2. `cloudflared.json` shows traffic per exposed hostname: request rate, status codes, latency percentiles, and traffic volume. These panels use the per hostname metrics from the default `ghcr.io/strrl/cloudflared` image.

Import each file in Grafana with `Dashboards -> New -> Import` and select your Prometheus data source. Both metrics endpoints described above must be scraped for the panels to show data.

Both dashboards are also published on grafana.com, so you can import them by ID instead of uploading a file:

1. [25659](https://grafana.com/grafana/dashboards/25659): Cloudflare Tunnel Ingress Controller
2. [25660](https://grafana.com/grafana/dashboards/25660): Cloudflare Tunnel Traffic by Hostname

## Load the Prometheus alert rules

The same directory ships `alerts.yaml` with alert rules for stale syncs, Cloudflare API errors, high 5xx rates per hostname, and unreachable origin services.

Add the file to the `rule_files` section of your Prometheus configuration, or wrap its `groups` list in a `PrometheusRule` object when using the Prometheus Operator. See the [mixin README](https://github.com/STRRL/cloudflare-tunnel-ingress-controller/tree/master/mixin) for the full alert list.

## Add cloudflared health probes

The chart copies `cloudflared.probes.liveness`, `readiness`, and `startup` into the managed connector container as Kubernetes probe objects.

This example uses the `cloudflared` readiness endpoint on the metrics port:

```yaml
cloudflared:
  probes:
    liveness:
      httpGet:
        path: /ready
        port: 44483
      initialDelaySeconds: 10
    readiness:
      httpGet:
        path: /ready
        port: 44483
    startup:
      httpGet:
        path: /ready
        port: 44483
      failureThreshold: 30
      periodSeconds: 2
```

Run your normal Helm upgrade, then inspect the managed Deployment and pods:

```bash
kubectl describe deployment controlled-cloudflared-connector \
  -n cloudflare-tunnel-ingress-controller

kubectl get pods \
  -n cloudflare-tunnel-ingress-controller \
  -l app=controlled-cloudflared-connector
```
