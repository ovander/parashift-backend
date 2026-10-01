#!/bin/bash

set -euo pipefail

echo "=============================="
echo "🚀 Parashift Backend Deployment START"
echo "Date: $(date -u)"
echo "=============================="

# -----------------------------
# CONFIG
# -----------------------------
APP_DIR="/opt/apps/parashift"
RELEASES_DIR="$APP_DIR/releases/backend"
CURRENT_LINK="$APP_DIR/current"
MIGRATIONS_DIR="$APP_DIR/migrations"
ENV_FILE="$APP_DIR/env/.env"

TMP_DIR="/tmp/parashift-backend"
TMP_BIN="$TMP_DIR/app"
TMP_MIGRATIONS="$TMP_DIR/migrations"

SERVICE="parashift"
USER="olivier"

# The API listens on 127.0.0.1:$PORT (PORT from the env file, default 4000,
# as in internal/config). Use 127.0.0.1, not localhost: localhost may resolve
# to ::1, which the IPv4 loopback bind refuses.
API_PORT="$(sudo sed -n 's/^[[:space:]]*PORT=\([0-9][0-9]*\).*/\1/p' "$ENV_FILE" | tail -n 1)"
API_BASE="http://127.0.0.1:${API_PORT:-4000}"
HEALTH_URL="$API_BASE/healthz"
VERSION_URL="$API_BASE/api/version"

VERSION="${1:-unknown}"
RELEASE_DIR="$RELEASES_DIR/$VERSION"

# -----------------------------
# ROLLBACK
# -----------------------------
rollback() {
    echo "❌ Deployment failed — rolling back..."

    if [ -n "${PREVIOUS:-}" ]; then
        sudo ln -sfn "$PREVIOUS" "$CURRENT_LINK"
        sudo systemctl start $SERVICE
        echo "✔ Rolled back to previous release"
    else
        echo "⚠️ No previous release to rollback"
    fi

    exit 1
}

trap rollback ERR

# -----------------------------
# PRECHECKS
# -----------------------------
echo "🔍 Pre-checks..."

[ -f "$TMP_BIN" ]        || { echo "❌ Missing binary in $TMP_BIN — run push.sh first"; exit 1; }
[ -d "$TMP_MIGRATIONS" ] || { echo "❌ Missing migrations in $TMP_MIGRATIONS"; exit 1; }

echo "✔ Pre-checks OK"

# -----------------------------
# STOP SERVICE
# -----------------------------
echo "🛑 Stopping service..."
sudo systemctl stop $SERVICE

# -----------------------------
# CREATE RELEASE
# -----------------------------
echo "📁 Creating release $VERSION..."

sudo mkdir -p "$RELEASE_DIR"
sudo mv "$TMP_BIN" "$RELEASE_DIR/app"

sudo chown -R $USER:$USER "$RELEASE_DIR"
sudo chmod +x "$RELEASE_DIR/app"

# -----------------------------
# SYNC MIGRATIONS
# -----------------------------
echo "📁 Syncing migrations..."

sudo rsync -av --delete "$TMP_MIGRATIONS/" "$MIGRATIONS_DIR/"
sudo chown -R $USER:$USER "$MIGRATIONS_DIR"
sudo chmod -R 755 "$MIGRATIONS_DIR"
sudo chmod 644 "$MIGRATIONS_DIR"/*.sql || true

# -----------------------------
# RUN MIGRATIONS
# -----------------------------
echo "🗄 Running migrations..."

sudo -u $USER bash -c "
set -a
source $ENV_FILE
set +a
$RELEASE_DIR/app migrate
"

# -----------------------------
# SWITCH RELEASE
# -----------------------------
echo "🔁 Switching release..."

PREVIOUS="$(readlink -f $CURRENT_LINK || echo "")"

sudo ln -sfn "$RELEASE_DIR" "$CURRENT_LINK"

# -----------------------------
# START SERVICE
# -----------------------------
echo "▶️ Starting service..."
sudo systemctl start $SERVICE

# -----------------------------
# CHECK SERVICE
# -----------------------------
if ! systemctl is-active --quiet $SERVICE; then
    echo "❌ Service failed to start"
    rollback
fi

# -----------------------------
# HEALTHCHECK
# -----------------------------
echo "🌐 Checking API..."

for i in {1..10}; do
    if curl -fs "$HEALTH_URL" > /dev/null; then
        echo "✔ API healthy"
        break
    fi
    sleep 1
done

if ! curl -fs "$HEALTH_URL" > /dev/null; then
    echo "❌ API healthcheck failed ($HEALTH_URL)"
    rollback
fi

# -----------------------------
# VERSION CHECK
# -----------------------------
echo "📦 Deployed version:"

# The binary has no "version" subcommand (it would start a second server);
# ask the running one instead.
curl -fs "$VERSION_URL" && echo || echo "⚠️ Version check failed ($VERSION_URL)"

# -----------------------------
# LOGS
# -----------------------------
echo "🔍 Service status:"
sudo systemctl status $SERVICE --no-pager

echo "📜 Recent logs:"
sudo journalctl -u $SERVICE -n 20 --no-pager

# -----------------------------
# CLEAN OLD RELEASES
# -----------------------------
echo "🧹 Cleaning old releases (keep last 5)..."

cd "$RELEASES_DIR"
ls -dt */ 2>/dev/null | tail -n +6 | xargs -r sudo rm -rf

# -----------------------------
# CLEANUP TMP
# -----------------------------
echo "🧹 Cleaning upload temp..."
rm -rf "$TMP_DIR"

# -----------------------------
# DONE
# -----------------------------
echo "=============================="
echo "✅ Deployment SUCCESS"
echo "Version: $VERSION"
echo "=============================="