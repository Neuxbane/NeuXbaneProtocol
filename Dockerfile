# syntax=docker/dockerfile:1

# ---------------------------------------------------------------------------
# NeuXbaneProtocol (nxp) — precompiled standalone server
#
# Everything is compiled at image-build time:
#   * nxp-server — the mother process, built from ./cmd/nxp-server
#   * backend    — the SINGLE worker binary compiled from define/**.go
#
# Every define/**/*.go handler is linked into one `backend` binary. There is no
# per-endpoint artifact. The runtime image therefore ships NO Go toolchain, NO
# source tree, NO shell and NO package manager — only the two static binaries
# on top of a distroless base (the lightest practical runtime).
# ---------------------------------------------------------------------------

# ---- Stage 1: build the mother + compile the worker backend ---------------
FROM golang:1.27-bookworm AS builder

WORKDIR /src

# Static, reproducible, no C toolchain required.
ENV CGO_ENABLED=0 \
    GOFLAGS=-mod=mod \
    GOTOOLCHAIN=local

# Framework source (mother + codegen + runtime).
COPY go.mod go.sum ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY nxp/ ./nxp/

RUN go mod download

# Build the mother binary. It also acts as the codegen driver
# (`nxp-server -build-backend`), so no extra tooling is needed.
RUN go build -trimpath -ldflags "-s -w" -o /out/nxp-server ./cmd/nxp-server

# ---- Stage 2: compile the worker backend from define/ ---------------------
# The standalone app tree (define/, local abi/, go.mod) is the developer
# workspace. It is compiled into the single `backend` worker binary here.
FROM builder AS backend

WORKDIR /app

# Standalone app workspace: define/ handlers + the local abi/ layer + go.mod.
COPY example/go.mod ./go.mod
COPY example/abi/ ./abi/
COPY example/define/ ./define/

# Generate routes from define/ and compile the single `backend` worker binary.
RUN /out/nxp-server -build-backend -define ./define -backend-out /out/backend \
    && ls -lh /out/backend

# ---- Stage 3: runtime -----------------------------------------------------
# distroless/static: no shell, no package manager, no libc, no Docker runtime.
# It ships only CA certificates + tzdata + a nonroot user — exactly what a
# static Go binary needs for outbound TLS and correct timestamps.
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

WORKDIR /app

COPY --from=builder /out/nxp-server /app/nxp-server
COPY --from=backend /out/backend    /app/backend

# Precompiled mode: spawn the shipped backend directly, no codegen / go build.
ENV NXP_ENV=prod \
    NXP_PRECOMPILED=1 \
    NXP_BACKEND_BIN=/app/backend \
    NXP_SOCK_DIR=/tmp/nxp-sockets \
    NXP_ADDR=:8080

# Persistent state lives here (mounted as a volume in production).
VOLUME ["/data"]

EXPOSE 8080

# distroless:nonroot already runs as uid 65532 (nonroot).
USER nonroot:nonroot

ENTRYPOINT ["/app/nxp-server"]
CMD ["-addr", ":8080", "-env", "prod"]
