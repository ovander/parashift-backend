#!/bin/bash
set -euo pipefail

# ==============================
# CONFIG
# ==============================
# The VPS address is not kept in the repository. Set SSH_USER, SSH_HOST and
# SSH_PORT in the environment or in ~/.config/parashift/deploy.env (another file
# with PARASHIFT_DEPLOY_ENV), for example:
#   SSH_USER=deploy
#   SSH_HOST=vps.example.com
#   SSH_PORT=22
DEPLOY_ENV="${PARASHIFT_DEPLOY_ENV:-${HOME}/.config/parashift/deploy.env}"
# shellcheck source=/dev/null
[ -f "${DEPLOY_ENV}" ] && . "${DEPLOY_ENV}"
: "${SSH_USER:?set SSH_USER (environment or ${DEPLOY_ENV})}"
: "${SSH_HOST:?set SSH_HOST (environment or ${DEPLOY_ENV})}"
SSH_PORT="${SSH_PORT:-22}"
REMOTE="${SSH_USER}@${SSH_HOST}"

APP_NAME="parashift"

VERSION="${1:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")"
BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

BIN_NAME="${APP_NAME}-${VERSION}"
LOCAL_BIN="/tmp/${BIN_NAME}"

REMOTE_TMP_DIR="/tmp/${APP_NAME}-backend"
REMOTE_BIN="${REMOTE_TMP_DIR}/app"
REMOTE_MIGRATIONS="${REMOTE_TMP_DIR}/migrations"

# ==============================
# GUARD
# ==============================
if [[ "${VERSION}" == *"-dirty"* ]]; then
  echo "❌ Working tree is dirty. Commit your changes before deploying."
  exit 1
fi

# ==============================
# BUILD
# ==============================
echo "=============================="
echo "🔨 Building ${BIN_NAME}"
echo "Version:  ${VERSION}"
echo "Commit:   ${COMMIT}"
echo "Time:     ${BUILD_TIME}"
echo "=============================="

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w \
    -X main.version=${VERSION} \
    -X main.commit=${COMMIT} \
    -X main.buildTime=${BUILD_TIME}" \
  -o "${LOCAL_BIN}" \
  ./cmd/server

# ==============================
# VALIDATION
# ==============================
echo "🔍 Validating binary..."

if [ ! -f "${LOCAL_BIN}" ]; then
  echo "❌ Build failed"
  exit 1
fi

chmod +x "${LOCAL_BIN}"
echo "✔ Binary built: ${LOCAL_BIN}"

# ==============================
# CHECKSUM
# ==============================
echo "🔐 Generating checksum..."
CHECKSUM=$(shasum -a 256 "${LOCAL_BIN}" | awk '{print $1}')
echo "Checksum: ${CHECKSUM}"

# ==============================
# UPLOAD BINARY
# ==============================
echo "📁 Preparing remote tmp..."
ssh -p ${SSH_PORT} ${REMOTE} "rm -rf ${REMOTE_TMP_DIR} && mkdir -p ${REMOTE_TMP_DIR}"

echo "📤 Uploading binary..."
scp -P ${SSH_PORT} "${LOCAL_BIN}" "${REMOTE}:${REMOTE_BIN}"

# ==============================
# VERIFY REMOTE
# ==============================
echo "🔍 Verifying remote binary..."
ssh -p ${SSH_PORT} ${REMOTE} "ls -lh ${REMOTE_BIN}"

# ==============================
# UPLOAD MIGRATIONS
# ==============================
echo "📁 Uploading migrations..."
rsync -az --delete -e "ssh -p ${SSH_PORT}" \
  migrations/ "${REMOTE}:${REMOTE_MIGRATIONS}/"

# ==============================
# CLEANUP LOCAL
# ==============================
echo "🧹 Cleaning local temp..."
rm -f "${LOCAL_BIN}"

# ==============================
# FINAL INSTRUCTIONS
# ==============================
echo ""
echo "=============================="
echo "✅ PUSH COMPLETE"
echo "=============================="
echo ""
echo "➡️  Next steps on VPS:"
echo ""
echo "    ssh -p ${SSH_PORT} ${REMOTE}"
echo ""
echo "    sudo /opt/apps/${APP_NAME}/deploy-backend.sh ${VERSION}"
echo ""
echo "=============================="