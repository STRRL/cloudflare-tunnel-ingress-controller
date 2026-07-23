---
title: Helm Values
description: Tune the controller and cloudflared connectors with chart values.
---

The `strrl.dev/cloudflare-tunnel-ingress-controller` chart exposes values for production hardening, observability, and connector behaviour. The tables below cover common settings and pod customization.

For the complete and up-to-date list of all available Helm values, refer to the [values.yaml](https://github.com/STRRL/cloudflare-tunnel-ingress-controller/blob/master/helm/cloudflare-tunnel-ingress-controller/values.yaml) file in the repository.

## Credentials and ingress

| Value                         | Default             | Notes                                                                                      |
| ----------------------------- | ------------------- | ------------------------------------------------------------------------------------------ |
| `cloudflare.apiToken`         | `""`                | Required when Helm creates the credential Secret.                                          |
| `cloudflare.accountId`        | `""`                | Required when Helm creates the credential Secret.                                          |
| `cloudflare.tunnelName`       | `""`                | Required when Helm creates the credential Secret.                                          |
| `cloudflare.secretRef.*`      | unset               | Use an existing Secret. Set `name`, `accountIDKey`, `tunnelNameKey`, and `apiTokenKey`.    |
| `ingressClass.name`           | `cloudflare-tunnel` | Name of the `IngressClass` created and watched by the controller.                          |
| `ingressClass.isDefaultClass` | `false`             | Set to `true` only if Cloudflare Tunnel should handle ingresses without an explicit class. |

## Controller pods

These values apply to the controller Deployment, not the managed cloudflared connector Deployment.

| Value                | Default                                                              | Notes                                                                                        |
| -------------------- | -------------------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| `replicaCount`       | `1`                                                                  | Number of controller pods. Enable `leaderElection.enabled` when using more than one replica. |
| `resources`          | CPU requests and limits: `100m`; memory requests and limits: `128Mi` | Controller container resource requests and limits.                                           |
| `securityContext`    | `{}`                                                                 | Kubernetes container security context for the controller container.                          |
| `podSecurityContext` | `{}`                                                                 | Kubernetes pod security context for controller pods.                                         |
| `priorityClassName`  | unset                                                                | PriorityClass assigned to controller pods.                                                   |

## Managed cloudflared connector pods

The chart writes these values to the deployment customization file consumed by the controller.

| Value                                   | Default  | Notes                                                                                                                                                                               |
| --------------------------------------- | -------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `cloudflared.image.tag`                 | `latest` | Image tag for managed cloudflared connector pods.                                                                                                                                   |
| `cloudflared.replicaCount`              | `1`      | Number of cloudflared connector pods maintaining the tunnel.                                                                                                                        |
| `cloudflared.extraArgs`                 | `[]`     | Extra arguments passed to cloudflared, such as `--post-quantum`.                                                                                                                    |
| `cloudflared.resources`                 | `{}`     | Container resource requests and limits.                                                                                                                                             |
| `cloudflared.securityContext`           | `{}`     | Kubernetes container security context for the cloudflared container.                                                                                                                |
| `cloudflared.podSecurityContext`        | `{}`     | Kubernetes pod security context for connector pods.                                                                                                                                 |
| `cloudflared.podAntiAffinity`           | `false`  | Adds required pod anti-affinity across `kubernetes.io/hostname`. Ignored when `cloudflared.affinity` is set. Extra replicas stay pending if there are not enough schedulable nodes. |
| `cloudflared.topologySpreadConstraints` | `[]`     | Kubernetes topology spread constraints for connector pods.                                                                                                                          |
| `cloudflared.priorityClassName`         | unset    | PriorityClass assigned to connector pods.                                                                                                                                           |
| `cloudflared.probes.liveness`           | `{}`     | Kubernetes liveness probe for the cloudflared container.                                                                                                                            |
| `cloudflared.probes.readiness`          | `{}`     | Kubernetes readiness probe for the cloudflared container.                                                                                                                           |
| `cloudflared.probes.startup`            | `{}`     | Kubernetes startup probe for the cloudflared container.                                                                                                                             |
| `cloudflared.volumes`                   | `[]`     | Kubernetes volumes added to connector pods.                                                                                                                                         |
| `cloudflared.volumeMounts`              | `[]`     | Kubernetes volume mounts added to the cloudflared container.                                                                                                                        |
| `cloudflared.pdb.enabled`               | `false`  | Create a PodDisruptionBudget for connector pods.                                                                                                                                    |
| `cloudflared.pdb.minAvailable`          | unset    | Minimum available connector pods. Mutually exclusive with `cloudflared.pdb.maxUnavailable`.                                                                                         |
| `cloudflared.pdb.maxUnavailable`        | unset    | Maximum unavailable connector pods. Mutually exclusive with `cloudflared.pdb.minAvailable`.                                                                                         |

## Cloudflared ServiceMonitor

These values configure the Prometheus Operator `ServiceMonitor` for managed cloudflared connectors.

| Value                                         | Default | Notes                                                                                      |
| --------------------------------------------- | ------- | ------------------------------------------------------------------------------------------ |
| `cloudflaredServiceMonitor.create`            | `false` | Create the ServiceMonitor.                                                                 |
| `cloudflaredServiceMonitor.jobLabel`          | `""`    | Service label used as the Prometheus job name. Omitted from the ServiceMonitor when empty. |
| `cloudflaredServiceMonitor.interval`          | `""`    | Scrape interval. Omitted from the endpoint when empty.                                     |
| `cloudflaredServiceMonitor.scrapeTimeout`     | `""`    | Scrape timeout. Omitted from the endpoint when empty.                                      |
| `cloudflaredServiceMonitor.honorLabels`       | `false` | Preserve labels from scraped metrics when they conflict with server-side labels.           |
| `cloudflaredServiceMonitor.metricRelabelings` | `[]`    | Metric relabeling rules applied after scraping.                                            |
| `cloudflaredServiceMonitor.relabelings`       | `[]`    | Target relabeling rules applied before scraping.                                           |
| `cloudflaredServiceMonitor.labels`            | `{}`    | Additional labels added to the ServiceMonitor.                                             |
| `cloudflaredServiceMonitor.scheme`            | `http`  | Scheme used to scrape the metrics endpoint.                                                |
