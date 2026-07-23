# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Kubernetes Ingress Controller that integrates with Cloudflare Tunnel to expose Kubernetes services to the internet securely without requiring port forwarding or firewall configuration. It watches Kubernetes Ingress resources and automatically configures Cloudflare Tunnels to route traffic to the corresponding services.

For the data flow, DNS ownership model, and connector reconciliation design, see the [architecture explanation](https://tunnel.strrl.dev/explanation/architecture/).

## Architecture Orientation

- **IngressController** (`pkg/controller/ingress-controller.go`): Reconciles controlled Kubernetes Ingress resources
- **Ingress transformation** (`pkg/controller/transform.go`): Converts Ingress rules and Services into Exposure objects
- **Exposure** (`pkg/exposure/exposure.go`): Internal representation shared between Kubernetes and Cloudflare logic
- **TunnelClient** (`pkg/cloudflare-controller/tunnel-client.go`): Reconciles tunnel ingress rules and DNS records through the Cloudflare API
- **DNS ownership** (`pkg/cloudflare-controller/dns.go`): Plans CNAME and ownership TXT record changes
- **ControlledCloudflaredConnector** (`pkg/controller/controlled-cloudflared-connector.go`): Reconciles the managed cloudflared Secret and Deployment

## Development Commands

```bash
# Initial setup (installs pre-commit hooks via prek)
make setup

# Run unit tests
make unit-test

# Run a single test
go test -run TestFunctionName ./pkg/path/to/package/...

# Run integration tests (requires setup-envtest)
make integration-test

# Build Docker image
make image

# Development with live reload (also runs setup)
make dev
```

### Pre-commit Hooks

Pre-commit hooks are managed via [prek](https://prek.j178.dev/) (configured in `prek.toml`). Hooks run `gofmt`, `go vet`, and `golangci-lint` automatically before each commit. Run `make setup` after cloning to install them.

## Configuration

### Required Flags
- `--cloudflare-api-token`: Cloudflare API token with Zone:Zone:Read, Zone:DNS:Edit and Account:Cloudflare Tunnel:Edit permissions
- `--cloudflare-account-id`: Cloudflare account ID
- `--cloudflare-tunnel-name`: Name of the Cloudflare tunnel to manage
- `--ingress-class`: Ingress class name (default: "cloudflare-tunnel")
- `--controller-class`: Controller class name (default: "strrl.dev/cloudflare-tunnel-ingress-controller")
- `--namespace`: Namespace to execute cloudflared connector (default: "default")

### Supported Annotations
- `cloudflare-tunnel-ingress-controller.strrl.dev/proxy-ssl-verify`: Enable/disable SSL verification ("on" or "off", default: "off")
- `cloudflare-tunnel-ingress-controller.strrl.dev/backend-protocol`: Backend protocol (default: "http")
- `cloudflare-tunnel-ingress-controller.strrl.dev/http-host-header`: Set HTTP Host header for the local webserver
- `cloudflare-tunnel-ingress-controller.strrl.dev/origin-server-name`: Hostname on the origin server certificate
- `cloudflare-tunnel-ingress-controller.strrl.dev/disable-dns-management`: Disable Cloudflare DNS record (CNAME/TXT) management for the ingress while still configuring the tunnel ingress rule, so DNS can be delegated to an external system such as external-dns or a Cloudflare Load Balancer ("true" or "false", default "false")

## Testing Strategy

### Unit Tests
Located in `pkg/` directories alongside source files (e.g., `dns_test.go`, `transform_test.go`).

### Integration Tests
Located in `test/integration/` using Ginkgo/Gomega framework with envtest for Kubernetes API simulation. The `hack/install-setup-envtest.sh` script installs `setup-envtest` if not present.

## Deployment

Helm chart in `helm/cloudflare-tunnel-ingress-controller/`. Example dev configurations in `hack/dev/`.
