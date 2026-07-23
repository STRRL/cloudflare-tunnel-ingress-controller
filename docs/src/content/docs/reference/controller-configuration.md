---
title: Controller Configuration
description: Configure the controller through CLI flags or environment variables.
---

The controller reads its configuration from CLI flags. Every flag can also be supplied as an environment variable: uppercase the flag name and replace hyphens with underscores. When both are set, the CLI flag takes precedence.

```bash
# These are equivalent:
--cloudflare-api-token=xxx
CLOUDFLARE_API_TOKEN=xxx
```

The Helm chart wires the credential secret and chart values into these settings for you, so most Helm users never touch them directly. They are useful when running the controller outside Helm or when injecting configuration from an external secret manager.

## Available settings

| Flag                             | Environment variable           | Purpose                                                                                     |
| -------------------------------- | ------------------------------ | ------------------------------------------------------------------------------------------- |
| `--cloudflare-api-token`         | `CLOUDFLARE_API_TOKEN`         | Cloudflare API token, see [Cloudflare Credentials](/reference/cloudflare-credentials/).     |
| `--cloudflare-account-id`        | `CLOUDFLARE_ACCOUNT_ID`        | Account identifier that owns the tunnel.                                                    |
| `--cloudflare-tunnel-name`       | `CLOUDFLARE_TUNNEL_NAME`       | Tunnel name created or reused by the controller.                                            |
| `--ingress-class`                | `INGRESS_CLASS`                | Ingress class to watch, defaults to `cloudflare-tunnel`.                                    |
| `--controller-class`             | `CONTROLLER_CLASS`             | Controller class name, defaults to `strrl.dev/cloudflare-tunnel-ingress-controller`.        |
| `--namespace`                    | `NAMESPACE`                    | Namespace where the managed cloudflared connector runs.                                     |
| `--cloudflared-protocol`         | `CLOUDFLARED_PROTOCOL`         | Transport protocol for cloudflared, defaults to `auto`.                                     |
| `--cloudflared-extra-args`       | `CLOUDFLARED_EXTRA_ARGS`       | Extra arguments passed to the cloudflared command.                                          |
| `--cloudflared-image`            | `CLOUDFLARED_IMAGE`            | Container image for the managed cloudflared connector.                                      |
| `--cloudflared-image-pull-policy`| `CLOUDFLARED_IMAGE_PULL_POLICY`| Image pull policy for the connector pods.                                                   |
| `--cloudflared-replica-count`    | `CLOUDFLARED_REPLICA_COUNT`    | Number of cloudflared connector pods.                                                       |
| `--cloudflared-deployment-config`| `CLOUDFLARED_DEPLOYMENT_CONFIG`| Path to a JSON file with pod template customization for the connector Deployment.           |
| `--cluster-domain`               | `CLUSTER_DOMAIN`               | Kubernetes cluster domain used to build Service FQDNs, defaults to `cluster.local`.         |
| `--leader-elect`                 | `LEADER_ELECT`                 | Enable leader election when running more than one controller replica.                       |
| `--dns-comment-template`         | `DNS_COMMENT_TEMPLATE`         | Go template for DNS record comments, empty string disables comments.                        |
| `--log-level`                    | `LOG_LEVEL`                    | Numeric log verbosity.                                                                      |
