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

| Value                                   | Default                         | Notes                                                                                                                                                                               |
| --------------------------------------- | ------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `cloudflared.image.repository`          | `ghcr.io/strrl/cloudflared`      | Image repository for managed cloudflared connector pods.                                                                                                                            |
| `cloudflared.image.tag`                 | `2026.7.3-host-metrics.1`        | Image tag for managed cloudflared connector pods.                                                                                                                                   |
| `cloudflared.replicaCount`              | `1`                             | Number of cloudflared connector pods maintaining the tunnel.                                                                                                                        |
| `cloudflared.extraArgs`                 | `[]`                            | Extra arguments passed to cloudflared, such as `--post-quantum`.                                                                                                                    |
| `cloudflared.resources`                 | `{}`                            | Container resource requests and limits.                                                                                                                                             |
| `cloudflared.securityContext`           | `{}`                            | Kubernetes container security context for the cloudflared container.                                                                                                                |
| `cloudflared.podSecurityContext`        | `{}`                            | Kubernetes pod security context for connector pods.                                                                                                                                 |
| `cloudflared.podAntiAffinity`           | `false`                         | Adds required pod anti-affinity across `kubernetes.io/hostname`. Ignored when `cloudflared.affinity` is set. Extra replicas stay pending if there are not enough schedulable nodes. |
| `cloudflared.topologySpreadConstraints` | `[]`                            | Kubernetes topology spread constraints for connector pods.                                                                                                                          |
| `cloudflared.priorityClassName`         | unset                           | PriorityClass assigned to connector pods.                                                                                                                                           |
| `cloudflared.probes.liveness`           | `{}`                            | Kubernetes liveness probe for the cloudflared container.                                                                                                                            |
| `cloudflared.probes.readiness`          | `{}`                            | Kubernetes readiness probe for the cloudflared container.                                                                                                                           |
| `cloudflared.probes.startup`            | `{}`                            | Kubernetes startup probe for the cloudflared container.                                                                                                                             |
| `cloudflared.volumes`                   | `[]`                            | Kubernetes volumes added to connector pods.                                                                                                                                         |
| `cloudflared.volumeMounts`              | `[]`                            | Kubernetes volume mounts added to the cloudflared container.                                                                                                                        |
| `cloudflared.pdb.enabled`               | `false`                         | Create a PodDisruptionBudget for connector pods.                                                                                                                                    |
| `cloudflared.pdb.minAvailable`          | unset                           | Minimum available connector pods. Mutually exclusive with `cloudflared.pdb.maxUnavailable`.                                                                                         |
| `cloudflared.pdb.maxUnavailable`        | unset                           | Maximum unavailable connector pods. Mutually exclusive with `cloudflared.pdb.minAvailable`.                                                                                         |

## ServiceMonitor

These values configure the Prometheus Operator `ServiceMonitor` objects. One switch creates ServiceMonitors for both metrics endpoints: the controller and the managed cloudflared connectors.

| Value                              | Default | Notes                                                                                      |
| ---------------------------------- | ------- | ------------------------------------------------------------------------------------------ |
| `serviceMonitor.create`            | `false` | Create both ServiceMonitors. Requires the Prometheus Operator CRDs.                        |
| `serviceMonitor.labels`            | `{}`    | Additional labels added to both ServiceMonitors.                                           |
| `serviceMonitor.interval`          | `""`    | Scrape interval. Omitted from the endpoints when empty.                                    |
| `serviceMonitor.scrapeTimeout`     | `""`    | Scrape timeout. Omitted from the endpoints when empty.                                     |
| `serviceMonitor.metricRelabelings` | `[]`    | Metric relabeling rules applied after scraping.                                            |
| `serviceMonitor.relabelings`       | `[]`    | Target relabeling rules applied before scraping.                                           |
| `serviceMonitor.cloudflared.jobLabel` | `""` | Service label used as the Prometheus job name for the connector target. Omitted when empty. |
| `serviceMonitor.cloudflared.honorLabels` | `false` | Preserve labels from scraped connector metrics when they conflict with server-side labels. |
| `serviceMonitor.cloudflared.scheme` | `http` | Scheme used to scrape the connector metrics endpoint.                                      |

## Uninstall behaviour

The connector Deployment and the tunnel token Secret are created by the controller at runtime with an owner reference to the controller Deployment. Kubernetes garbage collection removes them when the release is uninstalled, so no cloudflared pods keep running against a stale tunnel.

External resources are never touched during uninstall, following the same model as other ingress controllers:

- The Cloudflare tunnel is kept. Tunnels are addressed by name, a reinstall with the same `cloudflare.tunnelName` reuses it. Delete it from the Cloudflare dashboard (or via API) when it is no longer needed.
- DNS records are cleaned up by the controller whenever an Ingress is deleted. Delete your Ingress resources before uninstalling if you want the records removed; records belonging to Ingresses that still exist at uninstall time stay behind together with the tunnel.
