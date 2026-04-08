#!/bin/bash
# push.sh — build a production binary and upload it to the VPS.
#
# Usage:
#   ./scripts/push.sh [version]
#
# Examples:
#   ./scripts/push.sh
#   ./scripts/push.sh 1.2.0
#
# After this script finishes, run on the VPS:
#   ./deploy.sh

set -euo pipefail

SSH_USER="olivier"
SSH_HOST="vandermoten.eu"
SSH_PORT="2222"
REMOTE="${SSH_USER}@${SSH_HOST}"

VERSION="${1:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")"
BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

echo "🔨 Building ParaShift ${VERSION} (${COMMIT}) for linux/amd64…"

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w \
    -X main.version=${VERSION} \
    -X main.commit=${COMMIT} \
    -X main.buildTime=${BUILD_TIME}" \
  -o /tmp/app \
  ./cmd/server

echo "📤 Uploading binary → ${REMOTE}:/tmp/app"
scp -P ${SSH_PORT} /tmp/app "${REMOTE}:/tmp/"

echo "📤 Uploading migrations → ${REMOTE}:/tmp/migrations"
rsync -az --delete -e "ssh -p ${SSH_PORT}" migrations/ "${REMOTE}:/tmp/migrations/"

echo ""
echo "✅ Done — now run on the VPS:"
echo "   ./deploy.sh"
