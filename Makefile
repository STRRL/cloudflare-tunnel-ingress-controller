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

.PHONY: integration-test
integration-test: setup-envtest
	KUBEBUILDER_ASSETS="$(shell setup-envtest use $(ENVTEST_K8S_VERSION) -p path)" CGO_ENABLED=1 go test -race -v -coverpkg=./... -coverprofile ./test/integration/cover.out ./test/integration/...

.PHONY: e2e-image
e2e-image:
	DOCKER_BUILDKIT=1 TARGETARCH=amd64 docker build --build-arg COVER=1 --build-arg RUNTIME_BASE=gcr.io/distroless/base-debian12:debug-nonroot -t $(E2E_CONTROLLER_IMAGE) -f ./image/cloudflare-tunnel-ingress-controller/Dockerfile .

.PHONY: e2e
e2e: e2e-image
	E2E_CONTROLLER_IMAGE=$(E2E_CONTROLLER_IMAGE) go test -v -timeout 30m ./test/e2e

.PHONY: setup-envtest
setup-envtest:
	bash ./hack/install-setup-envtest.sh
