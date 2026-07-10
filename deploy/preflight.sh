#!/usr/bin/env bash
# Boomerang install preflight — verify the host can run the backup appliance.
set -euo pipefail

PREFLIGHT_WARN_ONLY="${PREFLIGHT_WARN_ONLY:-0}"
PREFLIGHT_BUILD="${PREFLIGHT_BUILD:-0}"
PREFLIGHT_MIN_DISK_MB="${PREFLIGHT_MIN_DISK_MB:-1024}"
PREFLIGHT_RECOMMEND_DISK_GB="${PREFLIGHT_RECOMMEND_DISK_GB:-20}"
PREFLIGHT_MIN_RAM_MB="${PREFLIGHT_MIN_RAM_MB:-512}"

_preflight_fail() {
  echo "✗ $1" >&2
  exit 1
}

_preflight_warn() {
  echo "⚠ $1" >&2
}

_preflight_ok() {
  echo "✓ $1"
}

_preflight_need_root() {
  if [[ "$(id -u)" -ne 0 ]]; then
    _preflight_fail "Run as root: sudo ./install.sh"
  fi
  _preflight_ok "Running as root"
}

_preflight_linux() {
  local uname_s
  uname_s="$(uname -s)"
  if [[ "$uname_s" != "Linux" ]]; then
    _preflight_fail "Boomerang requires Linux (found: $uname_s)"
  fi
  _preflight_ok "Linux host"
}

_preflight_arch() {
  local arch
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64|aarch64|arm64) _preflight_ok "CPU architecture: $arch" ;;
    *) _preflight_fail "Unsupported architecture: $arch (need amd64 or arm64)" ;;
  esac
}

_preflight_systemd() {
  if ! command -v systemctl >/dev/null 2>&1; then
    _preflight_fail "systemd not found — use Docker or install on Debian/Ubuntu with systemd"
  fi
  if ! systemctl is-system-running >/dev/null 2>&1; then
    _preflight_warn "systemd reports the system is not fully running — install may still work"
  else
    _preflight_ok "systemd is running"
  fi
}

_preflight_os() {
  if [[ ! -f /etc/os-release ]]; then
    _preflight_warn "Could not detect OS — Debian/Ubuntu are tested"
    return
  fi
  # shellcheck disable=SC1091
  . /etc/os-release
  case "${ID:-}" in
    debian|ubuntu)
      _preflight_ok "OS: ${PRETTY_NAME:-$ID}"
      ;;
    *)
      _preflight_warn "OS ${PRETTY_NAME:-$ID} is untested — Debian 12+ or Ubuntu 22.04+ recommended"
      ;;
  esac
}

_preflight_apt() {
  if ! command -v apt-get >/dev/null 2>&1; then
    _preflight_fail "apt-get not found — native install.sh targets Debian/Ubuntu (try Docker on other distros)"
  fi
  _preflight_ok "apt-get available"
}

_preflight_disk() {
  local data_dir="${BOOMERANG_DATA_DIR:-/var/lib/boomerang}"
  local check_path="$data_dir"
  local parent
  parent="$(dirname "$check_path")"
  while [[ ! -d "$parent" && "$parent" != "/" ]]; do
    parent="$(dirname "$parent")"
  done

  local free_kb free_mb free_gb
  free_kb="$(df -Pk "$parent" 2>/dev/null | awk 'NR==2 {print $4}')"
  if [[ -z "$free_kb" || ! "$free_kb" =~ ^[0-9]+$ ]]; then
    _preflight_warn "Could not measure free disk space for $data_dir"
    return
  fi
  free_mb=$((free_kb / 1024))
  free_gb=$((free_mb / 1024))

  if (( free_mb < PREFLIGHT_MIN_DISK_MB )); then
    _preflight_fail "Need at least ${PREFLIGHT_MIN_DISK_MB} MB free for $data_dir (found ~${free_mb} MB on $parent)"
  fi
  if (( free_gb < PREFLIGHT_RECOMMEND_DISK_GB )); then
    _preflight_warn "Only ~${free_gb} GB free on $parent — ${PREFLIGHT_RECOMMEND_DISK_GB} GB+ recommended for real retention"
  else
    _preflight_ok "Disk space: ~${free_gb} GB free on $parent"
  fi
}

_preflight_memory() {
  local mem_kb mem_mb
  mem_kb="$(awk '/MemAvailable:/ {print $2}' /proc/meminfo 2>/dev/null || true)"
  if [[ -z "$mem_kb" || ! "$mem_kb" =~ ^[0-9]+$ ]]; then
    _preflight_warn "Could not read available memory"
    return
  fi
  mem_mb=$((mem_kb / 1024))
  if (( mem_mb < PREFLIGHT_MIN_RAM_MB )); then
    _preflight_warn "Low memory (~${mem_mb} MB available) — ${PREFLIGHT_MIN_RAM_MB} MB+ recommended"
  else
    _preflight_ok "Memory: ~${mem_mb} MB available"
  fi
}

_preflight_port() {
  if command -v ss >/dev/null 2>&1; then
    if ss -tln 2>/dev/null | awk '{print $4}' | grep -qE ':8080$'; then
      _preflight_warn "Port 8080 is already in use — Boomerang may fail to start"
      return
    fi
  fi
  _preflight_ok "Port 8080 appears free"
}

_preflight_build_tools() {
  local missing=0
  if ! command -v go >/dev/null 2>&1; then
    if [[ "$(id -u)" -eq 0 ]]; then
      _preflight_ok "Go not installed yet — installer will try to add golang-go"
    else
      _preflight_warn "Go not found — run with sudo so build dependencies can be installed"
      missing=1
    fi
  else
    _preflight_ok "Go: $(go version | awk '{print $3}')"
  fi
  if ! command -v npm >/dev/null 2>&1; then
    if [[ "$(id -u)" -eq 0 ]]; then
      _preflight_ok "Node/npm not installed yet — installer will try to add nodejs/npm"
    else
      _preflight_warn "npm not found — run with sudo so build dependencies can be installed"
      missing=1
    fi
  else
    _preflight_ok "Node: $(node -v 2>/dev/null || echo unknown)"
  fi
  if (( missing == 1 && PREFLIGHT_WARN_ONLY == 0 )); then
    _preflight_fail "Build tools missing — use: sudo ./install.sh   or   ./install.sh --no-build /path/to/boomerang"
  fi
}

# Usage: boomerang_preflight [--build]
boomerang_preflight() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --build) PREFLIGHT_BUILD=1; shift ;;
      *) shift ;;
    esac
  done

  echo "==> Boomerang system check"
  _preflight_linux
  _preflight_arch
  if [[ "$PREFLIGHT_BUILD" == 1 ]]; then
    _preflight_build_tools
  fi
  if [[ "$PREFLIGHT_WARN_ONLY" != 1 ]]; then
    _preflight_need_root
    _preflight_os
    _preflight_apt
    _preflight_systemd
    _preflight_disk
    _preflight_memory
    _preflight_port
  fi
  echo "==> System check passed"
}
