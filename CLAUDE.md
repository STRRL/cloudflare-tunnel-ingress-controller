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
1. Ingress resources are created with `ingressClassName: cloudflare-tunnel`
2. IngressController reconciles changes and extracts exposure configurations
3. TunnelClient updates Cloudflare tunnel ingress rules and DNS CNAME records
4. ControlledCloudflaredConnector ensures cloudflared pods are running to maintain the tunnel

### Key Packages
- `pkg/controller/`: Kubernetes controllers and reconciliation logic
- `pkg/cloudflare-controller/`: Cloudflare API client and tunnel management
- `pkg/exposure/`: Data structures for representing service exposures

## Development Commands

### Building and Testing
```bash
# Run unit tests
make unit-test

# Run integration tests (requires setup-envtest)
make integration-test

# Build Docker image
make image

# Development with live reload
make dev
```

### Go Commands
```bash
# Standard Go operations
go mod tidy
go fmt ./...
go vet ./...

# Run tests with coverage
CGO_ENABLED=1 go test -race ./pkg/... -coverprofile ./cover.out
```

### Development Environment
```bash
# Start development environment with Skaffold
skaffold dev --namespace cloudflare-tunnel-ingress-controller-dev
```

## Configuration

### Required Environment Variables/Flags
- `--cloudflare-api-token`: Cloudflare API token with Zone:DNS:Edit and Account:Cloudflare Tunnel:Edit permissions
- `--cloudflare-account-id`: Cloudflare account ID
- `--cloudflare-tunnel-name`: Name of the Cloudflare tunnel to manage
- `--ingress-class`: Ingress class name (default: "cloudflare-tunnel")
- `--controller-class`: Controller class name (default: "strrl.dev/cloudflare-tunnel-ingress-controller")

### Supported Annotations
The controller recognizes standard Kubernetes ingress annotations and custom annotations for Cloudflare-specific features like `http-host-header` and `origin-server-name`.

## Testing Strategy

### Unit Tests
Located in `pkg/` directories alongside source files (e.g., `dns_test.go`, `transform_test.go`)

### Integration Tests
Located in `test/integration/` using Ginkgo/Gomega framework with envtest for Kubernetes API simulation

### Test Environment Setup
Integration tests use `setup-envtest` to download and configure a local Kubernetes API server for testing.

## Deployment

### Helm Chart
The project includes a Helm chart in `helm/cloudflare-tunnel-ingress-controller/` for easy deployment to Kubernetes clusters.

### Development Files
Example configurations are available in `hack/dev/` for local development and testing.

## Dependencies

- **Kubernetes**: Uses controller-runtime framework for Kubernetes integration
- **Cloudflare Go SDK**: Official Cloudflare API client for tunnel and DNS management
- **Cobra**: CLI framework for the main controller binary
- **Ginkgo/Gomega**: Testing framework for BDD-style tests