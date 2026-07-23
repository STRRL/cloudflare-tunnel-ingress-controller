---
title: Monitor the Controller and cloudflared
description: Scrape controller and cloudflared metrics and configure connector health probes.
---

The controller and its managed `cloudflared` connectors expose separate metrics endpoints.

See [Helm values](/reference/helm-values/) for chart defaults.

## Inspect controller metrics

The controller serves controller runtime metrics over HTTP at `/metrics` on port `8080`. The chart does not create a Service or ServiceMonitor for this endpoint.

Forward the port from one controller pod:

```bash
kubectl port-forward deployment/cloudflare-tunnel-ingress-controller \
  -n cloudflare-tunnel-ingress-controller \
  8080:8080
```

Read the endpoint from another terminal:

```bash
curl http://127.0.0.1:8080/metrics
```

Create your own Service and scrape configuration if Prometheus must collect controller metrics continuously.

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

When `cloudflaredServiceMonitor.create` is false, the Service carries `prometheus.io/scrape: "true"` and `prometheus.io/port: "44483"` annotations for annotation based discovery.

## Create a cloudflared ServiceMonitor

Prometheus Operator must be installed before you enable this object. Add the following settings to the values file used by the existing release:

```yaml
cloudflaredServiceMonitor:
  create: true
  labels:
    release: kube-prometheus-stack
  jobLabel: app.kubernetes.io/component
  scheme: http
  interval: 30s
  scrapeTimeout: 10s
  honorLabels: false
  metricRelabelings: []
  relabelings: []
```

Adjust each field for your Prometheus installation:

1. `create` renders the ServiceMonitor.
2. `labels` adds metadata labels, commonly used by a Prometheus selector.
3. `jobLabel` names the selected Service label used as the Prometheus job label.
4. `scheme` sets the scrape scheme.
5. `interval` sets the scrape interval.
6. `scrapeTimeout` sets the timeout for one scrape.
7. `honorLabels` controls whether labels from scraped metrics take precedence.
8. `metricRelabelings` adds metric relabel rules after a scrape.
9. `relabelings` adds target relabel rules before a scrape.

The generated ServiceMonitor selects only the managed connector Service in the Helm release namespace and scrapes its named `metrics` port at `/metrics`.

Run your normal Helm upgrade, then verify the object:

```bash
kubectl get servicemonitor \
  -n cloudflare-tunnel-ingress-controller
```

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
