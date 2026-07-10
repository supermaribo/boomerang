#!/usr/bin/env bash
# Install or upgrade the Boomerang binary and systemd unit (run as root).
set -euo pipefail

PREFIX="${PREFIX:-/usr/local}"
DATA_DIR="${BOOMERANG_DATA_DIR:-/var/lib/boomerang}"
BIN_SRC="${1:-}"

if [[ "$(id -u)" -ne 0 ]]; then
  echo "Run as root: sudo $0 /path/to/boomerang" >&2
  exit 1
fi

if [[ -z "$BIN_SRC" || ! -f "$BIN_SRC" ]]; then
  echo "Usage: $0 /path/to/boomerang" >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=preflight.sh
source "$SCRIPT_DIR/preflight.sh"
boomerang_preflight

export DEBIAN_FRONTEND=noninteractive

echo "==> Installing packages (SSH, rsync, MySQL client, mail)"
apt-get update -qq

# Postfix: local-only delivery for alert emails (non-interactive).
if ! dpkg -l postfix >/dev/null 2>&1; then
  debconf-set-selections <<< "postfix postfix/main_mailer_type string 'Local only'"
  debconf-set-selections <<< "postfix postfix/root_address string root"
fi

apt-get install -y -qq \
  openssh-client \
  rsync \
  default-mysql-client \
  ca-certificates \
  postfix \
  mailutils \
  >/dev/null

echo "==> Creating boomerang system user and data directories"
id -u boomerang >/dev/null 2>&1 || useradd --system --home "$DATA_DIR" --shell /usr/sbin/nologin boomerang
# Allow local mail via sendmail fallback (primary path uses SMTP to 127.0.0.1:25).
getent group postdrop >/dev/null 2>&1 && usermod -aG postdrop boomerang 2>/dev/null || true
install -d -m 700 -o boomerang -g boomerang \
  "$DATA_DIR" \
  "$DATA_DIR/secrets" \
  "$DATA_DIR/backups"

SETUP_TOKEN=""
if [[ ! -f "$DATA_DIR/app.db" ]]; then
  SETUP_TOKEN="$(openssl rand -hex 16)"
  printf '%s\n' "$SETUP_TOKEN" >"$DATA_DIR/secrets/setup.token"
  chmod 600 "$DATA_DIR/secrets/setup.token"
  chown boomerang:boomerang "$DATA_DIR/secrets/setup.token"
fi

echo "==> Installing binary to $PREFIX/bin/boomerang"
install -m 755 "$BIN_SRC" "$PREFIX/bin/boomerang"

install -m 644 "$SCRIPT_DIR/boomerang.service" /etc/systemd/system/boomerang.service

systemctl daemon-reload
systemctl enable boomerang.service
systemctl restart boomerang.service

sleep 1
if systemctl is-active --quiet boomerang.service; then
  echo "==> Boomerang is running"
else
  echo "==> Service failed to start — check: journalctl -u boomerang -n 30" >&2
  systemctl --no-pager --full status boomerang.service || true
  exit 1
fi

IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
echo
echo "Boomerang installed."
echo "  UI:      http://${IP:-localhost}:8080"
echo "  Data:    $DATA_DIR"
echo "  Logs:    journalctl -u boomerang -f"
echo
if [[ -n "$SETUP_TOKEN" ]]; then
  echo "Setup token (required on first visit): $SETUP_TOKEN"
  echo "  (also in $DATA_DIR/secrets/setup.token)"
  echo
fi
echo "Open the UI and set your admin password on first visit."
