.PHONY: dev
dev:
	skaffold dev --namespace cloudflare-tunnel-ingress-controller-dev

.PHONY: image
image:
	DOCKER_BUILDKIT=1 docker build -t ghcr.io/oliverbaehler/cloudflare-tunnel-ingress-controller -f ./image/cloudflare-tunnel-ingress-controller/Dockerfile . 

.PHONY: unit-test
unit-test:
	CGO_ENABLED=1 go test -race ./pkg/... -coverprofile ./cover.out

.PHONY: integration-test
integration-test: setup-envtest
	KUBEBUILDER_ASSETS="$(shell setup-envtest use $(ENVTEST_K8S_VERSION) -p path)" CGO_ENABLED=1 go test -race -v -coverpkg=./... -coverprofile ./test/integration/cover.out ./test/integration/...

.PHONY: setup-envtest
setup-envtest:
	bash ./hack/install-setup-envtest.sh

# Helm
SRC_ROOT = $(shell git rev-parse --show-toplevel)

helm-docs: HELMDOCS_VERSION := v1.11.0
helm-docs: docker
	@docker run -v "$(SRC_ROOT):/helm-docs" jnorwood/helm-docs:$(HELMDOCS_VERSION) --chart-search-root /helm-docs

helm-lint: CT_VERSION := v3.3.1
helm-lint: docker
	@docker run -v "$(SRC_ROOT):/workdir" --entrypoint /bin/sh quay.io/helmpack/chart-testing:$(CT_VERSION) -c "cd /workdir; ct lint --config ct.yaml --lint-conf lintconf.yaml --all --debug"

docker:
	@hash docker 2>/dev/null || {\
		echo "You need docker" &&\
		exit 1;\
	}%
