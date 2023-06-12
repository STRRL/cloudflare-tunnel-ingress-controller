.PHONY: dev
dev:
	skaffold dev --namespace cloudflare-tunnel-ingress-controller-dev

.PHONY: image
image:
	DOCKER_BUILDKIT=1 docker build -t ghcr.io/strrl/cloudflare-tunnel-ingress-controller -f ./image/cloudflare-tunnel-ingress-controller/Dockerfile . 
