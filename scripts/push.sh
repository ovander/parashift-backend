#!/bin/bash
set -euo pipefail

SSH_USER="olivier"
SSH_HOST="vandermoten.eu"
SSH_PORT="2222"
REMOTE="${SSH_USER}@${SSH_HOST}"

VERSION="${1:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")"
BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

BIN_NAME="parashift-${VERSION}"
LOCAL_BIN="/tmp/${BIN_NAME}"
REMOTE_BIN="/tmp/parashift-app"
REMOTE_MIGRATIONS="/tmp/migrations"

echo "=============================="
echo "🔨 Building ${BIN_NAME}"
echo "Version: ${VERSION}"
echo "Commit: ${COMMIT}"
echo "Time: ${BUILD_TIME}"
echo "=============================="

# -----------------------------
# BUILD
# -----------------------------
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w \
    -X main.version=${VERSION} \
    -X main.commit=${COMMIT} \
    -X main.buildTime=${BUILD_TIME}" \
  -o "${LOCAL_BIN}" \
  ./cmd/server

# -----------------------------
# VALIDATION
# -----------------------------
echo "🔍 Validating binary..."

if [ ! -f "${LOCAL_BIN}" ]; then
    echo "❌ Build failed"
    exit 1
fi

chmod +x "${LOCAL_BIN}"

echo "✔ Binary built: ${LOCAL_BIN}"

# -----------------------------
# CHECKSUM
# -----------------------------
echo "🔐 Generating checksum..."
CHECKSUM=$(shasum -a 256 "${LOCAL_BIN}" | awk '{print $1}')
echo "Checksum: ${CHECKSUM}"

# -----------------------------
# UPLOAD BINARY
# -----------------------------
echo "📤 Uploading binary..."

scp -P ${SSH_PORT} "${LOCAL_BIN}" "${REMOTE}:${REMOTE_BIN}"

# -----------------------------
# VERIFY REMOTE
# -----------------------------
echo "🔍 Verifying remote binary..."

ssh -p ${SSH_PORT} ${REMOTE} "
ls -lh ${REMOTE_BIN}
"

# -----------------------------
# UPLOAD MIGRATIONS
# -----------------------------
echo "📁 Uploading migrations..."

rsync -az --delete -e "ssh -p ${SSH_PORT}" \
  migrations/ "${REMOTE}:${REMOTE_MIGRATIONS}/"

# -----------------------------
# CLEANUP LOCAL
# -----------------------------
echo "🧹 Cleaning local temp..."
rm -f "${LOCAL_BIN}"

echo ""
echo "=============================="
echo "✅ READY TO DEPLOY"
echo "Run on VPS:"
echo "cd /opt/apps/parashift && sudo ./deploy.sh"
echo "=============================="