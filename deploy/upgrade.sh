#!/usr/bin/env bash
# Upgrade Boomerang on a systemd appliance (no git clone required).
set -euo pipefail

RELEASE_TAG="${1:-latest}"
GITHUB_REPO="${BOOMERANG_GITHUB_REPO:-supermaribo/boomerang}"
RAW_BASE="https://raw.githubusercontent.com/${GITHUB_REPO}/main"
PREFIX="${PREFIX:-/usr/local}"
DATA_DIR="${BOOMERANG_DATA_DIR:-/var/lib/boomerang}"

if [[ "$(id -u)" -ne 0 ]]; then
  echo "Run as root: sudo $0 [TAG]" >&2
  exit 1
fi

case "$(uname -m)" in
  x86_64 | amd64) asset="boomerang-linux-amd64" ;;
  aarch64 | arm64) asset="boomerang-linux-arm64" ;;
  *)
    echo "Unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

base="https://github.com/${GITHUB_REPO}/releases"
if [[ "$RELEASE_TAG" == "latest" ]]; then
  url="${base}/latest/download/${asset}"
  label="latest"
else
  tag="${RELEASE_TAG#v}"
  url="${base}/download/v${tag}/${asset}"
  label="v${tag}"
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

echo "==> Downloading ${asset} (${label})"
echo "    ${url}"
curl -fsSL --retry 3 --retry-delay 2 -o "${tmpdir}/boomerang" "$url"
chmod 755 "${tmpdir}/boomerang"

echo "==> Refreshing update helper and systemd unit"
curl -fsSL "${RAW_BASE}/deploy/boomerang-update" -o "${PREFIX}/sbin/boomerang-update"
chmod 755 "${PREFIX}/sbin/boomerang-update"
curl -fsSL "${RAW_BASE}/deploy/boomerang.service" -o /etc/systemd/system/boomerang.service

if command -v visudo >/dev/null 2>&1; then
  cat >/etc/sudoers.d/boomerang-update <<'EOF'
boomerang ALL=(root) NOPASSWD: /usr/local/sbin/boomerang-update *
EOF
  chmod 440 /etc/sudoers.d/boomerang-update
  if ! visudo -cf /etc/sudoers.d/boomerang-update >/dev/null 2>&1; then
    rm -f /etc/sudoers.d/boomerang-update
    echo "warning: could not install sudoers for in-app updates" >&2
  fi
fi

echo "==> Installing binary"
"${PREFIX}/sbin/boomerang-update" "${tmpdir}/boomerang"

systemctl daemon-reload

if id boomerang >/dev/null 2>&1 && sudo -u boomerang sudo -n "${PREFIX}/sbin/boomerang-update" --check >/dev/null 2>&1; then
  echo "==> In-app updates (Settings → Updates): OK"
else
  echo "==> warning: in-app updates may not work — check /etc/sudoers.d/boomerang-update" >&2
fi

ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
echo
echo "Boomerang upgraded (${label})."
echo "  UI:   http://${ip:-localhost}:8080"
echo "  Logs: journalctl -u boomerang -f"
