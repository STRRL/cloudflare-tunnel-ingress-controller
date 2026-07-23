---
title: Controller Configuration
description: Configure the controller through CLI flags or environment variables.
---

The controller reads its configuration from CLI flags. Every flag can also be supplied as an environment variable: uppercase the flag name and replace hyphens with underscores. When both are set, the CLI flag takes precedence. The table below lists every supported controller flag.

```bash
# These are equivalent:
--cloudflare-api-token=xxx
CLOUDFLARE_API_TOKEN=xxx
```

The Helm chart wires the credential secret and chart values into these settings for you, so most Helm users never touch them directly. They are useful when running the controller outside Helm or when injecting configuration from an external secret manager.

## Available settings

| Flag                              | Environment variable            | Default                                                                     | Description                                                                                                                                                          |
| --------------------------------- | ------------------------------- | --------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `--cloudflare-api-token`          | `CLOUDFLARE_API_TOKEN`          | (required)                                                                  | Cloudflare API token. See [Cloudflare Credentials](/reference/cloudflare-credentials/).                                                                              |
| `--cloudflare-account-id`         | `CLOUDFLARE_ACCOUNT_ID`         | (required)                                                                  | Account identifier that owns the tunnel.                                                                                                                             |
| `--cloudflare-tunnel-name`        | `CLOUDFLARE_TUNNEL_NAME`        | (required)                                                                  | Tunnel name created or reused by the controller.                                                                                                                     |
| `--ingress-class`                 | `INGRESS_CLASS`                 | `cloudflare-tunnel`                                                         | Ingress class name watched by the controller.                                                                                                                        |
| `--controller-class`              | `CONTROLLER_CLASS`              | `strrl.dev/cloudflare-tunnel-ingress-controller`                            | Controller class name used in `IngressClass.spec.controller`.                                                                                                        |
| `--log-level`, `-v`               | `LOG_LEVEL`                     | `0`                                                                         | Numeric log verbosity. `-v` is the shorthand for `--log-level` and accepts the same integer value.                                                                   |
| `--namespace`                     | `NAMESPACE`                     | `default`                                                                   | Namespace where the managed cloudflared connector runs.                                                                                                              |
| `--cloudflared-protocol`          | `CLOUDFLARED_PROTOCOL`          | `auto`                                                                      | Transport protocol used by cloudflared.                                                                                                                              |
| `--cloudflared-extra-args`        | `CLOUDFLARED_EXTRA_ARGS`        | (empty)                                                                     | Extra arguments passed to the cloudflared command.                                                                                                                   |
| `--cloudflared-image`             | `CLOUDFLARED_IMAGE`             | `cloudflare/cloudflared:latest`                                             | Container image for the managed cloudflared connector.                                                                                                               |
| `--cloudflared-image-pull-policy` | `CLOUDFLARED_IMAGE_PULL_POLICY` | `IfNotPresent`                                                              | Image pull policy for the managed connector pods.                                                                                                                    |
| `--cloudflared-replica-count`     | `CLOUDFLARED_REPLICA_COUNT`     | `1`                                                                         | Number of managed cloudflared connector pods.                                                                                                                        |
| `--cloudflared-deployment-config` | `CLOUDFLARED_DEPLOYMENT_CONFIG` | (empty)                                                                     | Path to a JSON file with pod template customization for the managed connector Deployment.                                                                            |
| `--cluster-domain`                | `CLUSTER_DOMAIN`                | `cluster.local`                                                             | Kubernetes cluster domain used to build Service FQDNs.                                                                                                               |
| `--leader-elect`                  | `LEADER_ELECT`                  | `false`                                                                     | Enable leader election for high availability.                                                                                                                        |
| `--dns-comment-template`          | `DNS_COMMENT_TEMPLATE`          | `managed by cloudflare-tunnel-ingress-controller, tunnel [{{.TunnelName}}]` | Go template for DNS record comments. Set it to an empty string to disable comments. Available variables are `{{.TunnelName}}`, `{{.TunnelId}}`, and `{{.Hostname}}`. |
