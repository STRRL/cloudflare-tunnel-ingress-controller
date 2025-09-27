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

### Building and Testing
```bash
# Run unit tests
make unit-test

# Run integration tests (requires setup-envtest)
make integration-test

# Run E2E tests (requires minikube and Cloudflare credentials)
make e2e-test          # Setup environment + run tests
make e2e-test-full     # Setup + run + cleanup
make e2e-setup         # Setup environment only  
make e2e-run           # Run tests only (assumes setup done)
make e2e-cleanup       # Cleanup environment only

# Run all tests
make test-all

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

# Run unit tests with coverage (same as make unit-test)
CGO_ENABLED=1 go test -race ./pkg/... -coverprofile ./cover.out

# Run integration tests with coverage (requires setup-envtest)
KUBEBUILDER_ASSETS="$(shell setup-envtest use $(ENVTEST_K8S_VERSION) -p path)" CGO_ENABLED=1 go test -race -v -coverpkg=./... -coverprofile ./test/integration/cover.out ./test/integration/...
```

### Development Environment
```bash
# Start development environment with Skaffold
skaffold dev --namespace cloudflare-tunnel-ingress-controller-dev
```

## Configuration

### Required Environment Variables/Flags
- `--cloudflare-api-token`: Cloudflare API token with Zone:Zone:Read, Zone:DNS:Edit and Account:Cloudflare Tunnel:Edit permissions
- `--cloudflare-account-id`: Cloudflare account ID
- `--cloudflare-tunnel-name`: Name of the Cloudflare tunnel to manage
- `--ingress-class`: Ingress class name (default: "cloudflare-tunnel")
- `--controller-class`: Controller class name (default: "strrl.dev/cloudflare-tunnel-ingress-controller")
- `--namespace`: Namespace to execute cloudflared connector (default: "default")
- `--cloudflared-protocol`: Cloudflared protocol (default: "auto")
- `--cloudflared-extra-args`: Extra arguments to pass to cloudflared

### Optional Environment Variables for Cloudflared
- `CLOUDFLARED_IMAGE`: Docker image for cloudflared (default: "cloudflare/cloudflared:latest")
- `CLOUDFLARED_IMAGE_PULL_POLICY`: Image pull policy (default: "IfNotPresent")
- `CLOUDFLARED_REPLICA_COUNT`: Number of cloudflared replicas (default: 1)
- `ENVTEST_K8S_VERSION`: Kubernetes version for integration tests

### Supported Annotations
The controller recognizes standard Kubernetes ingress annotations and the following custom annotations:
- `cloudflare-tunnel-ingress-controller.strrl.dev/proxy-ssl-verify`: Enable/disable SSL verification ("on" or "off", default: "off")
- `cloudflare-tunnel-ingress-controller.strrl.dev/backend-protocol`: Backend protocol (default: "http")
- `cloudflare-tunnel-ingress-controller.strrl.dev/http-host-header`: Set HTTP Host header for the local webserver
- `cloudflare-tunnel-ingress-controller.strrl.dev/origin-server-name`: Hostname on the origin server certificate

## Testing Strategy

### Unit Tests
Located in `pkg/` directories alongside source files (e.g., `dns_test.go`, `transform_test.go`)

### Integration Tests
Located in `test/integration/` using Ginkgo/Gomega framework with envtest for Kubernetes API simulation

### E2E Tests
Located in `test/e2e/` for end-to-end testing with real Cloudflare APIs and minikube. These tests validate the complete workflow from Kubernetes Ingress creation to real domain accessibility.

#### E2E Test Setup
1. **Prerequisites**: Install `minikube`, `helm`, and `kubectl`
2. **Configuration**: Copy `test/e2e/.env.example` to `test/e2e/.env` and configure:
   - `CLOUDFLARE_API_TOKEN`: API token with Zone:Zone:Read, Zone:DNS:Edit, Account:Cloudflare Tunnel:Edit permissions
   - `CLOUDFLARE_ACCOUNT_ID`: Your Cloudflare account ID
   - `CLOUDFLARE_TUNNEL_NAME`: Test tunnel name (will be created if doesn't exist)
   - `CLOUDFLARE_TEST_DOMAIN_SUFFIX`: Domain suffix for test hostnames (e.g., `test.yourdomain.com`)
   - `E2E_TESTS_ENABLED=true`: Enable E2E tests
3. **Run Tests**:
   - `make e2e-test-full`: Complete workflow (setup + test + cleanup)
   - `make e2e-test`: Setup + test (leaves environment for debugging)
   - `make e2e-setup`: Setup environment only
   - `make e2e-run`: Run tests only (assumes setup done)
   - `make e2e-cleanup`: Cleanup environment only

#### E2E Test Environment
The E2E setup automatically:
- **Starts minikube** if not running
- **Builds controller image** with `:dev` tag
- **Loads image** into minikube cluster
- **Deploys controller** via Helm with test configuration
- **Creates credentials** secret for Cloudflare API
- **Waits for controller** to be ready

#### E2E Test Requirements
- **minikube, helm, kubectl** installed
- **Real Cloudflare account** with API credentials
- **Domain ownership** for the test domain suffix
- **Docker** for building controller image

### Test Environment Setup
Integration tests use `setup-envtest` to download and configure a local Kubernetes API server for testing. The `hack/install-setup-envtest.sh` script automatically installs `setup-envtest` if not present. Tests use Ginkgo/Gomega with controller-runtime's envtest framework.

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