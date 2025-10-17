# syntax=docker/dockerfile:1.5

# Build stage
FROM golang:1.21 AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /workspace

COPY go.mod ./

# Copy application sources
COPY pkg ./pkg
COPY cmd ./cmd

# Build the operator binary
RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,target=/root/.cache/go-build,sharing=locked \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags='-s -w' -o /workspace/bin/configpropagation ./cmd/configpropagation

# Final stage
FROM gcr.io/distroless/base-debian12:nonroot

WORKDIR /
COPY --from=builder /workspace/bin/configpropagation /manager

USER nonroot:nonroot
ENTRYPOINT ["/manager"]
