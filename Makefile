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
	E2E_CONTROLLER_IMAGE=$(E2E_CONTROLLER_IMAGE) bash ./test/e2e/e2e.sh

.PHONY: setup-envtest
setup-envtest:
	bash ./hack/install-setup-envtest.sh

.PHONY: setup-controller-gen
setup-controller-gen:
	bash ./hack/install-controller-gen.sh

# Generate deepcopy functions for the API types.
.PHONY: generate
generate: setup-controller-gen
	controller-gen object paths=./pkg/apis/...

# Generate the CRD manifest into the helm chart. The keep policy makes
# helm leave the CRD and its objects in place on uninstall. The
# crds.install gate lets extra releases in the same cluster skip the
# CRD, because only one release can own a cluster scoped resource.
.PHONY: manifests
manifests: setup-controller-gen
	controller-gen crd paths=./pkg/apis/... output:crd:dir=./helm/cloudflare-tunnel-ingress-controller/templates/crds
	@for f in ./helm/cloudflare-tunnel-ingress-controller/templates/crds/*.yaml; do \
		awk '/controller-gen.kubebuilder.io\/version/ {print; print "    helm.sh/resource-policy: keep"; next} {print}' $$f > $$f.tmp; \
		printf '{{- if .Values.crds.install }}\n' > $$f; \
		cat $$f.tmp >> $$f; \
		printf '{{- end }}\n' >> $$f; \
		rm $$f.tmp; \
	done

# Compile the Grafana dashboards and Prometheus alert rules from jsonnet
# sources into plain files under mixin/dist. Requires the jsonnet binary.
.PHONY: dashboards
dashboards:
	jsonnet mixin/dashboards/controller.jsonnet > mixin/dist/controller.json
	jsonnet mixin/dashboards/cloudflared.jsonnet > mixin/dist/cloudflared.json
	jsonnet -S mixin/alerts/alerts.jsonnet > mixin/dist/alerts.yaml
