#!/usr/bin/env bash
# Boomerang — build (optional), download a release, and install on Debian / Ubuntu.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD=1
FROM_RELEASE=0
RELEASE_TAG="latest"
BINARY=""
ARCH="${ARCH:-$(uname -m)}"
GOOS="${GOOS:-linux}"
GITHUB_REPO="${BOOMERANG_GITHUB_REPO:-supermaribo/boomerang}"

usage() {
  cat <<EOF
Usage: sudo ./install.sh [options] [path/to/boomerang-binary]

Install Boomerang as a systemd service on Debian or Ubuntu.

Options:
  --from-release [TAG]  Download a GitHub release binary (default tag: latest)
  --release TAG         Same as --from-release TAG
  --no-build            Skip compile step; requires a pre-built binary path
  --binary PATH         Use this binary instead of building
  -h, --help            Show this help

Environment:
  BOOMERANG_DATA_DIR       Data directory (default: /var/lib/boomerang)
  BOOMERANG_GITHUB_REPO    GitHub owner/repo for release downloads
  PREFIX                   Install prefix for binary (default: /usr/local)
  GOOS / GOARCH            Cross-compile target (default: linux / host arch)

Examples:
  # Fresh install from latest GitHub release (recommended on appliances):
  git clone https://github.com/${GITHUB_REPO}.git
  cd boomerang
  sudo ./install.sh --from-release

  # Install a specific release:
  sudo ./install.sh --from-release v0.1.0

  # Build from source (development):
  sudo ./install.sh

  # Install a binary you built elsewhere:
  sudo ./install.sh --no-build ./dist/boomerang
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --from-release)
      FROM_RELEASE=1
      BUILD=0
      shift
      if [[ $# -gt 0 && "$1" != --* ]]; then
        RELEASE_TAG="$1"
        shift
      fi
      ;;
    --release)
      FROM_RELEASE=1
      BUILD=0
      RELEASE_TAG="${2:?--release requires a tag like v0.1.0}"
      shift 2
      ;;
    --no-build) BUILD=0; shift ;;
    --binary) BINARY="$2"; BUILD=0; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    -*) echo "Unknown option: $1" >&2; usage >&2; exit 1 ;;
    *)
      if [[ -z "$BINARY" ]]; then
        BINARY="$1"
        BUILD=0
      else
        echo "Unexpected argument: $1" >&2
        exit 1
      fi
      shift
      ;;
  esac
done

case "$ARCH" in
  x86_64|amd64) GOARCH=amd64 ;;
  aarch64|arm64) GOARCH=arm64 ;;
  *) echo "Unsupported architecture: $ARCH (set GOARCH manually)" >&2; exit 1 ;;
esac

# shellcheck source=deploy/preflight.sh
source "$ROOT/deploy/preflight.sh"
if [[ "$BUILD" == 1 ]]; then
  PREFLIGHT_WARN_ONLY=1 boomerang_preflight --build
else
  boomerang_preflight
fi

download_release() {
  local tag="$1"
  local asset="boomerang-linux-${GOARCH}"
  local base="https://github.com/${GITHUB_REPO}/releases"
  local url
  if [[ "$tag" == "latest" ]]; then
    url="${base}/latest/download/${asset}"
  else
    tag="${tag#v}"
    url="${base}/download/v${tag}/${asset}"
  fi
  mkdir -p "$ROOT/dist"
  echo "==> Downloading ${asset} (${tag})"
  echo "    ${url}"
  if ! curl -fsSL --retry 3 --retry-delay 2 -o "$ROOT/dist/boomerang" "$url"; then
    echo "Failed to download release asset." >&2
    echo "Check that tag exists and asset ${asset} is attached to the GitHub release." >&2
    exit 1
  fi
  chmod 755 "$ROOT/dist/boomerang"
  BINARY="$ROOT/dist/boomerang"
}

if [[ "$FROM_RELEASE" == 1 ]]; then
  download_release "$RELEASE_TAG"
elif [[ "$BUILD" == 1 ]]; then
  echo "==> Building Boomerang ($GOOS/$GOARCH)"
  export DEBIAN_FRONTEND=noninteractive
  if [[ "$(id -u)" -eq 0 ]]; then
    apt-get update -qq
    apt-get install -y -qq golang-go nodejs npm ca-certificates git >/dev/null 2>&1 || true
  fi
  if ! command -v go >/dev/null 2>&1; then
    echo "Go is required to build. Install golang, use --from-release, or pass --no-build with a binary." >&2
    exit 1
  fi
  if ! command -v npm >/dev/null 2>&1; then
    echo "Node.js/npm is required to build the UI." >&2
    exit 1
  fi
  mkdir -p "$ROOT/dist"
  echo "    npm run build (web UI)"
  (cd "$ROOT/web" && npm ci && npm run build)
  echo "    go build"
  VERSION="$(git -C "$ROOT" describe --tags --always --dirty 2>/dev/null || echo dev)"
  LDFLAGS="-s -w -X github.com/boomerang-backup/boomerang/internal/version.Version=${VERSION}"
  (cd "$ROOT" && CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -ldflags="$LDFLAGS" -o "$ROOT/dist/boomerang" ./cmd/boomerang)
  BINARY="$ROOT/dist/boomerang"
fi

if [[ -z "$BINARY" || ! -f "$BINARY" ]]; then
  echo "No binary found. Build failed, download failed, or pass --no-build with a binary path." >&2
  exit 1
fi

if [[ ! -f "$ROOT/deploy/install.sh" ]]; then
  echo "Missing deploy/install.sh — run from a cloned boomerang repository." >&2
  exit 1
fi

echo "==> Installing system service"
exec bash "$ROOT/deploy/install.sh" "$BINARY"
