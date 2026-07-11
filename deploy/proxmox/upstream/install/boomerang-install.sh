#!/usr/bin/env bash

# Copyright (c) 2026 Matt Bodell
# Author: SuperMaribo
# License: MIT | https://github.com/community-scripts/ProxmoxVED/raw/main/LICENSE
# Source: https://github.com/supermaribo/boomerang

source /dev/stdin <<<"$FUNCTIONS_FILE_PATH"
color
verb_ip6
catch_errors
setting_up_container
network_check
update_os

msg_info "Installing Dependencies"
if ! dpkg -l postfix >/dev/null 2>&1; then
  debconf-set-selections <<< "postfix postfix/main_mailer_type string 'Local only'"
  debconf-set-selections <<< "postfix postfix/root_address string root"
fi
$STD apt-get install -y \
  openssh-client \
  rsync \
  default-mysql-client \
  ca-certificates \
  postfix \
  mailutils \
  sudo \
  openssl
msg_ok "Installed Dependencies"

msg_info "Creating User and Directories"
id -u boomerang >/dev/null 2>&1 || useradd --system --home /var/lib/boomerang --shell /usr/sbin/nologin boomerang
getent group postdrop >/dev/null 2>&1 && usermod -aG postdrop boomerang 2>/dev/null || true
install -d -m 700 -o boomerang -g boomerang \
  /var/lib/boomerang \
  /var/lib/boomerang/secrets \
  /var/lib/boomerang/backups \
  /var/lib/boomerang/.update
msg_ok "Created User and Directories"

fetch_and_deploy_gh_release "boomerang" "supermaribo/boomerang" "singlefile" "latest" "/usr/local/bin" "boomerang-linux-$(arch_resolve)"

msg_info "Creating Update Helper"
cat <<'EOF' >/usr/local/sbin/boomerang-update
#!/usr/bin/env bash
set -euo pipefail

PREFIX="${PREFIX:-/usr/local}"
SERVICE="${BOOMERANG_SERVICE:-boomerang.service}"
DATA_DIR="${BOOMERANG_DATA_DIR:-/var/lib/boomerang}"

if [[ "${1:-}" == "--check" ]]; then
  [[ -x "${PREFIX}/bin/boomerang" ]] || exit 1
  [[ -x "${0}" ]] || exit 1
  systemctl cat "$SERVICE" >/dev/null 2>&1 || exit 1
  [[ -d "$DATA_DIR/.update" ]] || exit 1
  exit 0
fi

[[ "$(id -u)" -eq 0 ]] || exit 1
NEW="${1:-}"
[[ -n "$NEW" && -f "$NEW" ]] || exit 1
install -m 755 "$NEW" "$PREFIX/bin/boomerang"
systemctl restart "$SERVICE"
EOF
chmod 755 /usr/local/sbin/boomerang-update

if command -v visudo >/dev/null 2>&1; then
  cat >/etc/sudoers.d/boomerang-update <<'EOF'
boomerang ALL=(root) NOPASSWD: /usr/local/sbin/boomerang-update /var/lib/boomerang/.update/*
EOF
  chmod 440 /etc/sudoers.d/boomerang-update
  visudo -cf /etc/sudoers.d/boomerang-update >/dev/null 2>&1 || rm -f /etc/sudoers.d/boomerang-update
fi
msg_ok "Created Update Helper"

msg_info "Creating Service"
cat <<'EOF' >/etc/systemd/system/boomerang.service
[Unit]
Description=Boomerang backup appliance
After=network.target

[Service]
Type=simple
User=boomerang
Group=boomerang
Environment=BOOMERANG_DATA_DIR=/var/lib/boomerang
Environment=BOOMERANG_LISTEN=0.0.0.0:8080
ExecStart=/usr/local/bin/boomerang
Restart=on-failure
RestartSec=3
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/boomerang
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF
systemctl enable -q --now boomerang
msg_ok "Created Service"

motd_ssh
customize
cleanup_lxc
