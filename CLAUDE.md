# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Kubernetes Ingress Controller that integrates with Cloudflare Tunnel to expose Kubernetes services to the internet securely without requiring port forwarding or firewall configuration. It watches Kubernetes Ingress resources and automatically configures Cloudflare Tunnels to route traffic to the corresponding services.

## Key Architecture Components

### Core Controllers
- **IngressController** (`pkg/controller/ingress-controller.go`): Main reconciler that watches Ingress resources and manages tunnel configurations
- **TunnelClient** (`pkg/cloudflare-controller/tunnel-client.go`): Handles Cloudflare API interactions for tunnel configuration and DNS management
- **ControlledCloudflaredConnector** (`pkg/controller/controlled-cloudflared-connector.go`): Manages cloudflared daemon deployment in Kubernetes

### Data Flow
1. Ingress resources are created with `ingressClassName: cloudflare-tunnel` or the annotation `kubernetes.io/ingress.class: cloudflare-tunnel`
2. IngressController reconciles changes and transforms Ingress specs into Exposure objects (`pkg/exposure/exposure.go`)
3. TunnelClient updates Cloudflare tunnel ingress rules and creates/updates DNS CNAME records pointing to the tunnel domain
4. ControlledCloudflaredConnector runs every 10 seconds to ensure cloudflared pods are deployed and up-to-date
5. Cloudflared connects to Cloudflare and maintains the tunnel, routing traffic based on the configured ingress rules

### Key Packages
- `pkg/controller/`: Kubernetes controllers and reconciliation logic
- `pkg/cloudflare-controller/`: Cloudflare API client and tunnel management
- `pkg/exposure/`: Data structures for representing service exposures

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
- `cloudflare-tunnel-ingress-controller.strrl.dev/backend-protocol`: Backend protocol, "http" or "https" (default: "http", case-insensitive)
- `cloudflare-tunnel-ingress-controller.strrl.dev/http-host-header`: Set HTTP Host header for the local webserver
- `cloudflare-tunnel-ingress-controller.strrl.dev/origin-server-name`: Hostname on the origin server certificate

## Testing Strategy

### Unit Tests
Located in `pkg/` directories alongside source files (e.g., `dns_test.go`, `transform_test.go`).

### Integration Tests
Located in `test/integration/` using Ginkgo/Gomega framework with envtest for Kubernetes API simulation. The `hack/install-setup-envtest.sh` script installs `setup-envtest` if not present.

## Deployment

Helm chart in `helm/cloudflare-tunnel-ingress-controller/`. Example dev configurations in `hack/dev/`.
