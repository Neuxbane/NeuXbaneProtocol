#!/usr/bin/env bash
#
# compile.sh — compile the nxp-server (mother) binary from source
#
# Usage:
#   ./compile.sh                 # build ./nxp-server (host OS/arch)
#   ./compile.sh --linux         # cross-compile a static linux/amd64 binary
#   ./compile.sh --out bin/nxp   # custom output path
#   ./compile.sh --debug         # keep symbols (no -s -w), no trimpath
#
# The result is a static (CGO_ENABLED=0) binary that runs on a distroless /
# scratch runtime with no Go toolchain present.
#
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
OUT="${OUT:-server}"
GOOS_TARGET="${GOOS:-}"
GOARCH_TARGET="${GOARCH:-}"
LDFLAGS="-s -w"
TRIMPATH="-trimpath"

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------
while [[ $# -gt 0 ]]; do
    case "$1" in
        --linux)
            GOOS_TARGET="linux"
            GOARCH_TARGET="${GOARCH_TARGET:-amd64}"
            shift
            ;;
        --out)
            OUT="$2"
            shift 2
            ;;
        --debug)
            LDFLAGS=""
            TRIMPATH=""
            shift
            ;;
        -h|--help)
            sed -n '2,14p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *)
            echo "unknown option: $1" >&2
            exit 1
            ;;
    esac
done

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
log()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
ok()   { printf '\033[1;32m  ✓\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m  ✗ %s\033[0m\n' "$*" >&2; exit 1; }

# ---------------------------------------------------------------------------
# Preflight
# ---------------------------------------------------------------------------
command -v go >/dev/null 2>&1 || die "go toolchain not found in PATH"

cd "$(dirname "$0")"

[[ -f go.mod ]] || die "go.mod not found in $(pwd)"
[[ -d cmd/nxp-server ]] || die "cmd/nxp-server not found in $(pwd)"

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------
mkdir -p "$(dirname "$OUT")"

log "Compiling nxp-server -> ${OUT}"
if [[ -n "$GOOS_TARGET" ]]; then
    log "target: ${GOOS_TARGET}/${GOARCH_TARGET:-$(go env GOARCH)}"
fi

CGO_ENABLED=0 \
GOOS="${GOOS_TARGET:-$(go env GOOS)}" \
GOARCH="${GOARCH_TARGET:-$(go env GOARCH)}" \
    go build ${TRIMPATH} ${LDFLAGS:+-ldflags "$LDFLAGS"} -o "$OUT" ./cmd/nxp-server

ok "built: ${OUT} ($(du -h "$OUT" | cut -f1))"
