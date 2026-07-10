#!/usr/bin/env bash
# Boomerang — build (optional) and install on Debian / Ubuntu (LXC, VPS, bare metal).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD=1
BINARY=""
ARCH="${ARCH:-$(uname -m)}"
GOOS="${GOOS:-linux}"

usage() {
  cat <<'EOF'
Usage: sudo ./install.sh [options] [path/to/boomerang-binary]

Install Boomerang as a systemd service on Debian or Ubuntu.

Options:
  --no-build          Skip compile step; requires a pre-built binary path
  --binary PATH       Use this binary instead of building
  -h, --help          Show this help

Environment:
  BOOMERANG_DATA_DIR  Data directory (default: /var/lib/boomerang)
  PREFIX              Install prefix for binary (default: /usr/local)
  GOOS / GOARCH       Cross-compile target (default: linux / host arch)

Examples:
  # Clone repo on the appliance, then:
  sudo ./install.sh

  # Install a binary you built elsewhere:
  sudo ./install.sh --no-build ./dist/boomerang

  # Cross-compile from macOS, copy dist/boomerang, then on the server:
  sudo ./install.sh --no-build /tmp/boomerang
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
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

if [[ "$BUILD" == 1 ]]; then
  echo "==> Building Boomerang ($GOOS/$GOARCH)"
  export DEBIAN_FRONTEND=noninteractive
  if [[ "$(id -u)" -eq 0 ]]; then
  apt-get update -qq
    apt-get install -y -qq golang-go nodejs npm ca-certificates git >/dev/null 2>&1 || true
  fi
  if ! command -v go >/dev/null 2>&1; then
    echo "Go is required to build. Install golang or pass --no-build with a binary." >&2
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
  (cd "$ROOT" && CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -o "$ROOT/dist/boomerang" ./cmd/boomerang)
  BINARY="$ROOT/dist/boomerang"
fi

if [[ -z "$BINARY" || ! -f "$BINARY" ]]; then
  echo "No binary found. Build failed or pass --binary PATH." >&2
  exit 1
fi

echo "==> Installing system service"
exec bash "$ROOT/deploy/install.sh" "$BINARY"
