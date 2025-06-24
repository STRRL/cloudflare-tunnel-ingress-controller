.PHONY: help
help: ## Show this help message
	@echo "Cloudflare Tunnel Ingress Controller - Available Make Targets:"
	@echo ""
	@echo "Development:"
	@echo "  dev                Build and run controller in development mode with Skaffold"
	@echo "  image              Build Docker image"
	@echo ""
	@echo "Testing:"
	@echo "  unit-test          Run unit tests with coverage"
	@echo "  integration-test   Run integration tests with envtest (mocked Kubernetes API)"
	@echo "  test-all           Run unit and integration tests"
	@echo ""
	@echo "E2E Testing:"
	@echo "  e2e-setup          Setup E2E environment (minikube + controller deployment)"
	@echo "  e2e-run            Run E2E tests (assumes environment is setup)"
	@echo "  e2e-cleanup        Cleanup E2E environment"
	@echo "  e2e-test           Setup environment + run E2E tests"
	@echo "  e2e-test-full      Complete E2E workflow: setup + test + cleanup"
	@echo ""
	@echo "Utilities:"
	@echo "  setup-envtest      Install setup-envtest tool for integration tests"
	@echo "  help               Show this help message"
	@echo ""
	@echo "E2E Test Requirements:"
	@echo "  - Copy test/e2e/.env.example to test/e2e/.env and configure Cloudflare credentials"
	@echo "  - Install: minikube, helm, kubectl, docker"
	@echo "  - Ensure you own the domain specified in CLOUDFLARE_TEST_DOMAIN_SUFFIX"

.PHONY: dev
dev:
	skaffold dev --namespace cloudflare-tunnel-ingress-controller-dev

.PHONY: image
image:
	DOCKER_BUILDKIT=1 TARGETARCH=amd64 docker build -t ghcr.io/strrl/cloudflare-tunnel-ingress-controller -f ./image/cloudflare-tunnel-ingress-controller/Dockerfile . 

.PHONY: unit-test
unit-test:
	CGO_ENABLED=1 go test -race ./pkg/... -coverprofile ./cover.out

.PHONY: integration-test
integration-test: setup-envtest
	KUBEBUILDER_ASSETS="$(shell setup-envtest use $(ENVTEST_K8S_VERSION) -p path)" CGO_ENABLED=1 go test -race -v -coverpkg=./... -coverprofile ./test/integration/cover.out ./test/integration/...

.PHONY: e2e-setup
e2e-setup:
	@echo "Setting up E2E environment..."
	@command -v minikube >/dev/null || (echo "minikube required for E2E tests" && exit 1)
	@command -v helm >/dev/null || (echo "helm required for E2E tests" && exit 1)
	@test -f test/e2e/.env || (echo "Create test/e2e/.env file (see test/e2e/.env.example)" && exit 1)
	./test/e2e/setup.sh

.PHONY: e2e-run
e2e-run:
	@echo "Running E2E tests..."
	@test -f test/e2e/.env || (echo "Create test/e2e/.env file (see test/e2e/.env.example)" && exit 1)
	cd test/e2e && ACK_GINKGO_DEPRECATIONS=2.22.2 go test -v -timeout=15m -ginkgo.v -ginkgo.show-node-events .

.PHONY: e2e-cleanup
e2e-cleanup:
	@echo "Cleaning up E2E environment..."
	./test/e2e/cleanup.sh

.PHONY: e2e-test
e2e-test: e2e-setup e2e-run
	@echo "E2E tests completed successfully"

.PHONY: e2e-test-full
e2e-test-full: e2e-setup e2e-run e2e-cleanup
	@echo "Full E2E test cycle completed"

.PHONY: test-all
test-all: unit-test integration-test
	@echo "All tests completed"

.PHONY: setup-envtest
setup-envtest:
	bash ./hack/install-setup-envtest.sh
