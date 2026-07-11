#!/usr/bin/env bash
# Boomerang — Proxmox VE LXC helper (run on the Proxmox host).
# Creates a Debian LXC and installs Boomerang from the latest GitHub release.
#
# One-liner (paste into the Proxmox shell):
#   bash -c "$(curl -fsSL https://raw.githubusercontent.com/supermaribo/boomerang/main/deploy/proxmox/ct-boomerang.sh)"
#
# Until this script is listed on community-scripts.org, it fetches the install
# script from this repository instead of community-scripts/ProxmoxVE.

source <(curl -fsSL https://raw.githubusercontent.com/community-scripts/ProxmoxVE/main/misc/build.func)
# Copyright (c) 2026 Boomerang contributors
# License: AGPL-3.0 (Boomerang application) | MIT (community-scripts build helpers)
# Source: https://github.com/supermaribo/boomerang

APP="Boomerang"
var_tags="${var_tags:-backup;self-hosted;mysql}"
var_cpu="${var_cpu:-1}"
var_ram="${var_ram:-512}"
var_disk="${var_disk:-20}"
var_os="${var_os:-debian}"
var_version="${var_version:-12}"
var_arm64="${var_arm64:-yes}"
var_unprivileged="${var_unprivileged:-1}"

BOOMERANG_REPO="${BOOMERANG_REPO:-supermaribo/boomerang}"
BOOMERANG_BRANCH="${BOOMERANG_BRANCH:-main}"
BOOMERANG_INSTALL_URL="https://raw.githubusercontent.com/${BOOMERANG_REPO}/${BOOMERANG_BRANCH}/deploy/proxmox/boomerang-install.sh"

# community-scripts build.func always pulls install/*.sh from its own repo.
# Redirect only that request to our installer until the script is upstreamed.
_curl_real="$(command -v curl)"
curl() {
  local args=() url="" rewrite=0
  for arg in "$@"; do
    if [[ "$arg" == https://raw.githubusercontent.com/community-scripts/ProxmoxVE/main/install/boomerang-install.sh ]]; then
      url="$BOOMERANG_INSTALL_URL"
      rewrite=1
      continue
    fi
    args+=("$arg")
  done
  if (( rewrite )); then
    "$_curl_real" "${args[@]}" "$url"
  else
    "$_curl_real" "$@"
  fi
}

function update_script() {
  header_info
  check_container_storage
  check_container_resources

  if [[ ! -x /usr/local/bin/boomerang ]]; then
    msg_error "No ${APP} installation found!"
    exit
  fi

  local arch asset
  case "$(uname -m)" in
    x86_64 | amd64) arch="amd64" ;;
    aarch64 | arm64) arch="arm64" ;;
    *)
      msg_error "Unsupported CPU architecture: $(uname -m)"
      exit
      ;;
  esac
  asset="boomerang-linux-${arch}"

  if check_for_gh_release "boomerang" "${BOOMERANG_REPO}"; then
    msg_info "Stopping Boomerang"
    systemctl stop boomerang
    msg_ok "Stopped Boomerang"

    msg_info "Downloading ${CHECK_UPDATE_RELEASE}"
    fetch_and_deploy_gh_release "boomerang" "${BOOMERANG_REPO}" "singlefile" "latest" "/tmp" "${asset}"
    install -m 755 /tmp/boomerang /usr/local/bin/boomerang
    rm -f /tmp/boomerang

    msg_info "Starting Boomerang"
    systemctl start boomerang
    msg_ok "Started Boomerang"
    msg_ok "Updated successfully!"
  fi
  exit
}

header_info "$APP"
variables
color
catch_errors

start
build_container
description

msg_ok "Completed Successfully!\n"
echo -e "${CREATING}${GN}${APP} setup has been successfully initialized!${CL}"
echo -e "${INFO}${YW}Access it using the following URL:${CL}"
echo -e "${GATEWAY}${BGN}http://${IP}:8080${CL}"
echo -e "${INFO}${YW}First visit: open http://<container-ip>:8080 and set your admin password.${CL}"
