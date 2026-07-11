#!/usr/bin/env bash
# Copyright (c) 2026 Boomerang contributors
# License: AGPL-3.0 | https://github.com/supermaribo/boomerang
# Source: https://github.com/supermaribo/boomerang

source /dev/stdin <<<"$FUNCTIONS_FILE_PATH"
color
verb_ip6
catch_errors
setting_up_container
network_check
update_os

BOOMERANG_REPO="${BOOMERANG_REPO:-supermaribo/boomerang}"
BOOMERANG_BRANCH="${BOOMERANG_BRANCH:-main}"
RAW_BASE="https://raw.githubusercontent.com/${BOOMERANG_REPO}/${BOOMERANG_BRANCH}"
DATA_DIR="/var/lib/boomerang"
PREFIX="/usr/local"

if ! dpkg -l postfix >/dev/null 2>&1; then
  debconf-set-selections <<< "postfix postfix/main_mailer_type string 'Local only'"
  debconf-set-selections <<< "postfix postfix/root_address string root"
fi

msg_info "Installing packages (SSH, rsync, MySQL client, mail, sudo)"
$STD apt-get install -y \
  openssh-client \
  rsync \
  default-mysql-client \
  ca-certificates \
  postfix \
  mailutils \
  sudo \
  openssl \
  curl \
  >/dev/null
msg_ok "Installed packages"

msg_info "Creating boomerang system user and data directories"
id -u boomerang >/dev/null 2>&1 || useradd --system --home "$DATA_DIR" --shell /usr/sbin/nologin boomerang
getent group postdrop >/dev/null 2>&1 && usermod -aG postdrop boomerang 2>/dev/null || true
install -d -m 700 -o boomerang -g boomerang \
  "$DATA_DIR" \
  "$DATA_DIR/secrets" \
  "$DATA_DIR/backups" \
  "$DATA_DIR/.update"
msg_ok "Prepared data directories"

msg_info "Downloading Boomerang release"
case "$(uname -m)" in
  x86_64 | amd64) asset="boomerang-linux-amd64" ;;
  aarch64 | arm64) asset="boomerang-linux-arm64" ;;
  *)
    msg_error "Unsupported architecture: $(uname -m)"
    exit
    ;;
esac

fetch_and_deploy_gh_release "boomerang" "$BOOMERANG_REPO" "singlefile" "latest" "/tmp" "$asset"
install -m 755 /tmp/boomerang "$PREFIX/bin/boomerang"
rm -f /tmp/boomerang
msg_ok "Installed Boomerang binary"

msg_info "Installing update helper and systemd unit"
curl -fsSL "$RAW_BASE/deploy/boomerang-update" -o /usr/local/sbin/boomerang-update
chmod 755 /usr/local/sbin/boomerang-update
curl -fsSL "$RAW_BASE/deploy/boomerang.service" -o /etc/systemd/system/boomerang.service

if command -v visudo >/dev/null 2>&1; then
  cat >/etc/sudoers.d/boomerang-update <<'EOF'
boomerang ALL=(root) NOPASSWD: /usr/local/sbin/boomerang-update /var/lib/boomerang/.update/*
EOF
  chmod 440 /etc/sudoers.d/boomerang-update
  if ! visudo -cf /etc/sudoers.d/boomerang-update >/dev/null 2>&1; then
    rm -f /etc/sudoers.d/boomerang-update
    msg_warn "Could not install sudoers for in-app updates"
  fi
fi

systemctl daemon-reload
systemctl enable -q --now boomerang
msg_ok "Started Boomerang service"

if ! sudo -u boomerang sudo -n /usr/local/sbin/boomerang-update --check >/dev/null 2>&1; then
  msg_warn "In-app updates (Settings → Updates) may not work until the service unit is refreshed"
fi

motd_ssh
customize

cleanup_lxc
