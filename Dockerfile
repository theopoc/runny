# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e

FROM golang:1.27-alpine@sha256:b8c9b82affbbb82a020bbcf998dad2ba9516944ac7e883524642a481a9505e9c AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
ARG VERSION
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    test -n "${VERSION}" && \
    CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X github.com/theopoc/runny/internal/app.Version=${VERSION}" \
    -o /out/runny ./cmd/runny

FROM alpine:3.24@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a

RUN apk add --no-cache bash direnv zsh && \
    addgroup -S -g 1001 runny && \
    adduser -S -u 1001 -G runny -s /bin/zsh runny

COPY --from=builder /out/runny /usr/local/bin/runny

WORKDIR /workspace
USER runny
ENTRYPOINT ["runny"]
