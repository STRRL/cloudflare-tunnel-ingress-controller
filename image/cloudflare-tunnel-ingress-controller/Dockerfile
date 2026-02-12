# Global ARGs used in FROM instructions
ARG COVER=0
ARG RUNTIME_BASE=gcr.io/distroless/static:nonroot

# Build the manager binary
FROM golang:1.25 AS builder

ARG COVER
ARG TARGETARCH

WORKDIR /workspace

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Build
COPY  . .
RUN --mount=type=cache,target=/go \
  CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH GO111MODULE=on \
  sh -c ' \
    if [ "$COVER" = "1" ]; then \
      go build -cover -coverpkg=./... -o cloudflare-tunnel-ingress-controller ./cmd/cloudflare-tunnel-ingress-controller; \
    else \
      go build -ldflags="-s -w" -o cloudflare-tunnel-ingress-controller ./cmd/cloudflare-tunnel-ingress-controller; \
    fi \
  '

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details

# Runtime: production (no shell, no coverage)
FROM ${RUNTIME_BASE} AS runtime-cover-0

# Runtime: coverage-instrumented (debug image with shell and tar)
FROM ${RUNTIME_BASE} AS runtime-cover-1
RUN ["/busybox/sh", "-c", "mkdir -p /tmp/coverage"]
ENV GOCOVERDIR=/tmp/coverage

# Select runtime based on COVER arg
FROM runtime-cover-${COVER}
LABEL org.opencontainers.image.source=https://github.com/STRRL/cloudflare-tunnel-ingress-controller
WORKDIR /
COPY --from=builder /workspace/cloudflare-tunnel-ingress-controller /usr/bin/cloudflare-tunnel-ingress-controller
USER nonroot:nonroot

ENTRYPOINT ["cloudflare-tunnel-ingress-controller"]
