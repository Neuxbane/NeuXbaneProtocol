#!/usr/bin/env bash
#
# build.sh — build the NeuXbaneProtocol (nxp) Docker image and export it as a .tar.gz
#
# Usage:
#   ./build.sh                 # build + export nxp.tar.gz
#   ./build.sh --no-cache      # force a clean rebuild
#   ./build.sh --load          # also load the image into the local daemon
#   ./build.sh --tag v1.2.3    # override the image tag
#
# The exported archive can be shipped to an air-gapped host and imported with:
#   gunzip -c nxp.tar.gz | docker load
#
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
IMAGE_NAME="${IMAGE_NAME:-nxp}"
TAG="${IMAGE_NAME}:latest"
ARCHIVE="nxp.tar.gz"

DOCKER="${DOCKER:-docker}"

NO_CACHE=""
LOAD_IMAGE=0

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------
while [[ $# -gt 0 ]]; do
    case "$1" in
        --no-cache)
            NO_CACHE="--no-cache"
            shift
            ;;
        --load)
            LOAD_IMAGE=1
            shift
            ;;
        --tag)
            VERSION="$2"
            TAG="${IMAGE_NAME}:${VERSION}"
            ARCHIVE="nxp.tar.gz"
            shift 2
            ;;
        -h|--help)
            sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'
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
command -v "$DOCKER" >/dev/null 2>&1 || die "docker CLI not found (set DOCKER=... to override)"

cd "$(dirname "$0")"

[[ -f Dockerfile ]] || die "Dockerfile not found in $(pwd)"
[[ -f example/go.mod ]] || die "example/go.mod not found — the standalone app tree is required"

log "Building image ${TAG}"
"$DOCKER" build ${NO_CACHE} -t "${TAG}" .
ok "image built: ${TAG}"

# ---------------------------------------------------------------------------
# Export to .tar.gz
# ---------------------------------------------------------------------------
log "Exporting image to ${ARCHIVE}"
# `docker save` streams a tar; pipe straight through gzip to avoid a huge
# intermediate file on disk.
"$DOCKER" save "${TAG}" | gzip -9 > "${ARCHIVE}"
ok "archive written: ${ARCHIVE} ($(du -h "${ARCHIVE}" | cut -f1))"

# ---------------------------------------------------------------------------
# Optional: load back into the local daemon
# ---------------------------------------------------------------------------
if [[ "${LOAD_IMAGE}" -eq 1 ]]; then
    log "Loading image back into the local daemon"
    gunzip -c "${ARCHIVE}" | "$DOCKER" load
    ok "image loaded"
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
cat <<EOF

$(printf '\033[1;32mBuild complete.\033[0m')

  Image:    ${TAG}
  Archive:  ${ARCHIVE}

Run it:

  docker run -d --name nxp \\
    -p 8080:8080 \\
    -v nxp-data:/data \\
    ${TAG}

Import on another host:

  gunzip -c ${ARCHIVE} | docker load

EOF
