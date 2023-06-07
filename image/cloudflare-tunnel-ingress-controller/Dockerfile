# Build the manager binary
FROM golang:1.20 as builder

WORKDIR /workspace

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Build
COPY  . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -o cloudflare-tunnel-ingress-controller ./cmd/cloudflare-tunnel-ingress-controller

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
LABEL org.opencontainers.image.source=https://github.com/STRRL/cloudflare-tunnel-ingress-controller
WORKDIR /
COPY --from=builder /workspace/cloudflare-tunnel-ingress-controller .
USER nonroot:nonroot

ENTRYPOINT ["/cloudflare-tunnel-ingress-controller"]
