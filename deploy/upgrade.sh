#!/usr/bin/env bash
# Upgrade Boomerang on a systemd appliance (no git clone required).
set -euo pipefail

RELEASE_TAG="${1:-latest}"
GITHUB_REPO="${BOOMERANG_GITHUB_REPO:-supermaribo/boomerang}"
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
  sums_url="${base}/latest/download/SHA256SUMS"
  label="latest"
  RAW_BASE="https://raw.githubusercontent.com/${GITHUB_REPO}/main"
else
  tag="${RELEASE_TAG#v}"
  url="${base}/download/v${tag}/${asset}"
  sums_url="${base}/download/v${tag}/SHA256SUMS"
  label="v${tag}"
  RAW_BASE="https://raw.githubusercontent.com/${GITHUB_REPO}/v${tag}"
fi

staging_dir="${DATA_DIR}/.update"
mkdir -p "$staging_dir"
chown boomerang:boomerang "$staging_dir"
chmod 700 "$staging_dir"
tmpdir="$staging_dir"
trap 'rm -f "${tmpdir}/boomerang" "${tmpdir}/SHA256SUMS"' EXIT

echo "==> Downloading ${asset} (${label})"
echo "    ${url}"

max_wait=180
interval=10
elapsed=0
while true; do
  http_code="$(curl -sSL -o "${tmpdir}/boomerang" -w "%{http_code}" "$url" || true)"
  if [[ "$http_code" == "200" ]]; then
    break
  fi
  if [[ "$http_code" == "404" && "$elapsed" -lt "$max_wait" ]]; then
    echo "    Release asset not ready yet (GitHub Actions may still be building). Retrying in ${interval}s…"
    sleep "$interval"
    elapsed=$((elapsed + interval))
    continue
  fi
  rm -f "${tmpdir}/boomerang"
  echo "Failed to download release asset (HTTP ${http_code:-unknown})." >&2
  if [[ "$RELEASE_TAG" != "latest" ]]; then
    echo "New tags take 1–2 minutes to publish binaries. Wait and retry, or run without a tag for latest:" >&2
    echo "  curl -fsSL https://raw.githubusercontent.com/${GITHUB_REPO}/main/deploy/upgrade.sh | sudo bash" >&2
  fi
  exit 1
done
chmod 755 "${tmpdir}/boomerang"

if curl -fsSL -o "${tmpdir}/SHA256SUMS" "$sums_url" 2>/dev/null; then
  expected="$(awk -v f="$asset" '$2 == f || $2 == "*"f {print $1; exit}' "${tmpdir}/SHA256SUMS")"
  if [[ -n "$expected" ]]; then
    actual="$(sha256sum "${tmpdir}/boomerang" | awk '{print $1}')"
    if [[ "$actual" != "$expected" ]]; then
      echo "Checksum mismatch for ${asset}" >&2
      exit 1
    fi
    echo "==> Checksum verified"
  fi
else
  echo "==> warning: SHA256SUMS not found — skipping checksum verification" >&2
fi

echo "==> Refreshing update helper and systemd unit (${label})"
curl -fsSL "${RAW_BASE}/deploy/boomerang-update" -o "${PREFIX}/sbin/boomerang-update"
chmod 755 "${PREFIX}/sbin/boomerang-update"
curl -fsSL "${RAW_BASE}/deploy/boomerang.service" -o /etc/systemd/system/boomerang.service

if command -v visudo >/dev/null 2>&1; then
  cat >/etc/sudoers.d/boomerang-update <<EOF
boomerang ALL=(root) NOPASSWD: ${PREFIX}/sbin/boomerang-update ${DATA_DIR}/.update/*
EOF
  chmod 440 /etc/sudoers.d/boomerang-update
  if ! visudo -cf /etc/sudoers.d/boomerang-update >/dev/null 2>&1; then
    rm -f /etc/sudoers.d/boomerang-update
    echo "warning: could not install sudoers for in-app updates" >&2
  fi
fi

systemctl daemon-reload

echo "==> Installing binary"
"${PREFIX}/sbin/boomerang-update" "${tmpdir}/boomerang"
rm -f "${tmpdir}/boomerang" "${tmpdir}/SHA256SUMS"
chown boomerang:boomerang "$staging_dir"
chmod 700 "$staging_dir"

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
