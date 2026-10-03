E2E_CONTROLLER_IMAGE ?= cloudflare-tunnel-ingress-controller:e2e

.PHONY: setup
setup:
	@command -v prek >/dev/null 2>&1 || { echo "prek not found, install it from https://prek.j178.dev/installation/"; exit 1; }
	prek install

.PHONY: dev
dev: setup
	skaffold dev --namespace cloudflare-tunnel-ingress-controller-dev --cache-artifacts=false

.PHONY: image
image:
	DOCKER_BUILDKIT=1 TARGETARCH=amd64 docker build -t ghcr.io/strrl/cloudflare-tunnel-ingress-controller -f ./image/cloudflare-tunnel-ingress-controller/Dockerfile . 

.PHONY: unit-test
unit-test:
	CGO_ENABLED=1 go test -race ./pkg/... -coverprofile ./cover.out

# Fetch the envtest assets path in its own shell step, so a failed download
# stops the build instead of running the tests with an empty KUBEBUILDER_ASSETS.
.PHONY: integration-test
integration-test: setup-envtest
	set -e; \
	envtest_assets="$$(setup-envtest use $(ENVTEST_K8S_VERSION) -p path)"; \
	KUBEBUILDER_ASSETS="$$envtest_assets" CGO_ENABLED=1 go test -race -v -coverpkg=./... -coverprofile ./test/integration/cover.out ./test/integration/...

.PHONY: e2e-image
e2e-image:
	DOCKER_BUILDKIT=1 TARGETARCH=amd64 docker build --build-arg COVER=1 --build-arg RUNTIME_BASE=gcr.io/distroless/base-debian12:debug-nonroot -t $(E2E_CONTROLLER_IMAGE) -f ./image/cloudflare-tunnel-ingress-controller/Dockerfile .

.PHONY: e2e
e2e: e2e-image
	E2E_CONTROLLER_IMAGE=$(E2E_CONTROLLER_IMAGE) bash ./test/e2e/e2e.sh

.PHONY: setup-envtest
setup-envtest:
	bash ./hack/install-setup-envtest.sh

# Compile the Grafana dashboards from jsonnet sources into plain files
# under mixin/dist. Requires the jsonnet binary.
.PHONY: dashboards
dashboards:
	jsonnet mixin/dashboards/controller.jsonnet > mixin/dist/controller.json
	jsonnet mixin/dashboards/cloudflared.jsonnet > mixin/dist/cloudflared.json

# Gateway API conformance through the real Cloudflare edge, see
# test/conformance/conformance.sh. ENV_FILE points at the credentials
# (default ./.env.e2e), RUN_ID separates environments (default local-$USER).
# `make conformance RUN_TEST=HTTPRouteSimpleSameNamespace` runs one test.
.PHONY: conformance-up
conformance-up:
	bash ./test/conformance/conformance.sh up

.PHONY: conformance-deploy
conformance-deploy:
	bash ./test/conformance/conformance.sh deploy

.PHONY: conformance
conformance:
	RUN_TEST=$(RUN_TEST) bash ./test/conformance/conformance.sh run

.PHONY: conformance-down
conformance-down:
	bash ./test/conformance/conformance.sh down

.PHONY: conformance-clean
conformance-clean:
	bash ./test/conformance/conformance.sh clean
