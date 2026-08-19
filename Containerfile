# syntax=docker/dockerfile:1
# Containerfile — works with both podman and docker.
#
# Stages:
#   builder  — compiles the binary
#   test     — runs all unit tests (used by CI; never shipped)
#   final    — minimal distroless image containing only the binary

# ─── builder ────────────────────────────────────────────────────────────────
# scan-fix(trivy:CVE-2026-56860,CVE-2026-56862): bump base image — 1.25.8's
# stdlib carries the same net/url and crypto/tls CVEs already fixed by the
# go.mod toolchain bump; the container was still baking in the old runtime.
FROM golang:1.25.13-alpine AS builder

WORKDIR /src

RUN apk add --no-cache build-base git

# Download dependencies before copying source (improves layer caching).
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -trimpath \
      -ldflags="-w -s" \
      -o /bin/platform-runner \
      ./cmd/platform-runner

# ─── test ────────────────────────────────────────────────────────────────────
# This stage is only used in CI to run tests inside the same build environment.
# It is never pushed or run in production.
FROM builder AS test

RUN CGO_ENABLED=1 go test ./... -count=1 -race

# ─── final ───────────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot AS final

COPY --from=builder /bin/platform-runner /bin/platform-runner

ENTRYPOINT ["/bin/platform-runner"]
