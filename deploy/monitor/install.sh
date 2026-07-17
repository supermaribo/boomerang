#!/usr/bin/env bash
# Install boomerang-monitor on a Linux server (requires root / sudo).
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/supermaribo/boomerang/main/deploy/monitor/install.sh \
#     | sudo bash -s -- --public-key 'ssh-ed25519 AAAA…'
set -euo pipefail

REPO="${BOOMERANG_REPO:-supermaribo/boomerang}"
VERSION="${BOOMERANG_MONITOR_VERSION:-latest}"
INSTALL_USER="boomerang-monitor"
INSTALL_HOME="/var/lib/boomerang-monitor"
BIN_PATH="/usr/local/bin/boomerang-monitor"
PUBLIC_KEY=""
UNINSTALL=0

usage() {
  cat <<EOF
Usage: sudo bash install.sh --public-key 'ssh-ed25519 AAAA… comment'
       sudo bash install.sh --uninstall
Env:
  BOOMERANG_MONITOR_VERSION  release tag (default: latest)
  BOOMERANG_REPO             GitHub owner/repo (default: supermaribo/boomerang)
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --public-key)
      PUBLIC_KEY="${2:-}"
      shift 2
      ;;
    --uninstall)
      UNINSTALL=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown arg: $1" >&2
      usage
      exit 2
      ;;
  esac
done

if [[ "$(id -u)" -ne 0 ]]; then
  echo "must run as root (sudo)" >&2
  exit 1
fi

if [[ "$UNINSTALL" -eq 1 ]]; then
  systemctl disable --now boomerang-monitor.service 2>/dev/null || true
  rm -f /etc/systemd/system/boomerang-monitor.service
  systemctl daemon-reload || true
  rm -f "$BIN_PATH"
  if id "$INSTALL_USER" &>/dev/null; then
    userdel "$INSTALL_USER" 2>/dev/null || true
  fi
  rm -rf "$INSTALL_HOME"
  echo "boomerang-monitor uninstalled"
  exit 0
fi

if [[ -z "$PUBLIC_KEY" ]]; then
  echo "--public-key is required" >&2
  usage
  exit 2
fi
if [[ "$PUBLIC_KEY" != ssh-ed25519\ * && "$PUBLIC_KEY" != ssh-rsa\ * ]]; then
  echo "public key must start with ssh-ed25519 or ssh-rsa" >&2
  exit 2
fi

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *)
    echo "unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

if [[ "$VERSION" == "latest" ]]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep -oE '"tag_name":[[:space:]]*"[^"]+"' | head -1 | cut -d'"' -f4)"
fi
if [[ -z "$VERSION" ]]; then
  echo "could not resolve release version" >&2
  exit 1
fi

asset="boomerang-monitor-linux-${arch}"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT
base="https://github.com/${REPO}/releases/download/${VERSION}"
echo "==> Downloading ${asset} (${VERSION})"
curl -fsSL -o "${tmpdir}/${asset}" "${base}/${asset}"
curl -fsSL -o "${tmpdir}/SHA256SUMS" "${base}/SHA256SUMS"
( cd "$tmpdir" && grep " ${asset}\$" SHA256SUMS | sha256sum -c - )

install -m 755 "${tmpdir}/${asset}" "$BIN_PATH"

# The account needs a real shell: sshd runs the forced command via the login
# shell, so nologin would break metric export. Access stays restricted by the
# authorized_keys forced command + no-pty/no-forwarding options.
if ! id "$INSTALL_USER" &>/dev/null; then
  useradd --system --home-dir "$INSTALL_HOME" --create-home --shell /bin/sh "$INSTALL_USER"
else
  usermod -s /bin/sh "$INSTALL_USER"
fi
# journalctl read access for remote log viewing over the forced SSH key.
if getent group systemd-journal >/dev/null 2>&1; then
  usermod -aG systemd-journal "$INSTALL_USER"
fi
# Debian/Ubuntu grant read-only access to web and system log files via adm.
if getent group adm >/dev/null 2>&1; then
  usermod -aG adm "$INSTALL_USER"
fi
install -d -o "$INSTALL_USER" -g "$INSTALL_USER" -m 750 "$INSTALL_HOME"
install -d -o "$INSTALL_USER" -g "$INSTALL_USER" -m 750 "$INSTALL_HOME/.ssh"
install -d -o "$INSTALL_USER" -g "$INSTALL_USER" -m 750 "$INSTALL_HOME/spool"

forced="command=\"${BIN_PATH} ssh-forced\",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty"
auth="$INSTALL_HOME/.ssh/authorized_keys"
# Replace any previous Boomerang monitor key lines, keep others.
tmp_auth="$(mktemp)"
if [[ -f "$auth" ]]; then
  grep -v 'boomerang-monitor' "$auth" >"$tmp_auth" || true
fi
printf '%s %s boomerang-monitor\n' "$forced" "$PUBLIC_KEY" >>"$tmp_auth"
install -o "$INSTALL_USER" -g "$INSTALL_USER" -m 600 "$tmp_auth" "$auth"
rm -f "$tmp_auth"

cat >/etc/systemd/system/boomerang-monitor.service <<EOF
[Unit]
Description=Boomerang host metrics collector
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${INSTALL_USER}
Group=${INSTALL_USER}
Environment=BOOMERANG_MONITOR_SPOOL=${INSTALL_HOME}/spool
ExecStart=${BIN_PATH} daemon
Restart=always
RestartSec=10
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${INSTALL_HOME}
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now boomerang-monitor.service

echo
echo "boomerang-monitor ${VERSION} installed."
echo "  User:    ${INSTALL_USER}"
echo "  Binary:  ${BIN_PATH}"
echo "  Spool:   ${INSTALL_HOME}/spool"
echo "  Service: systemctl status boomerang-monitor"
echo
echo "In Boomerang, set SSH host/port/user to this server as ${INSTALL_USER}."
