#!/usr/bin/env bash
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

header_info "$APP"
variables
color
catch_errors

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

start
build_container
description

msg_ok "Completed Successfully!\n"
echo -e "${CREATING}${GN}${APP} setup has been successfully initialized!${CL}"
echo -e "${INFO}${YW}Access it using the following URL:${CL}"
echo -e "${GATEWAY}${BGN}http://${IP}:8080${CL}"
echo -e "${INFO}${YW}First visit: open http://${IP}:8080 and set your admin password.${CL}"
