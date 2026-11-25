#!/usr/bin/env bash
# Copyright (c) 2026 Ehs-Colin
# License: MIT
# Source: https://github.com/Ehs-Colin/what-game-should-we-play
#
# Installs Game Picker Utility (WGSWP) inside a Proxmox VE LXC container.
# App scripts live in deploy/proxmox; the engine comes from community-scripts/core.

# shellcheck disable=SC1090,SC1091
source /dev/stdin <<<"$FUNCTIONS_FILE_PATH"
color
verb_ip6
catch_errors

APP="${APPLICATION:-WGSWP}"

# ------------------------------------------------------------------------------
# Validation helpers
# ------------------------------------------------------------------------------

_wgswp_validate_username() {
  local username="$1"
  ((${#username} >= 3 && ${#username} <= 64)) || return 1
  [[ "$username" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]]
}

# systemd's EnvironmentFile parser understands double quotes and backslash
# escapes. Reject values that would change meaning or break the file rather
# than trying to escape them differently in the unit and the file.
_wgswp_validate_env_value() {
  local name="$1"
  local value="$2"

  if [[ -z "$value" ]]; then
    msg_error "${name} must not be empty"
    return 1
  fi
  if [[ "$value" == *'"'* || "$value" == *\\* ]]; then
    msg_error "${name} must not contain double quotes or backslashes"
    return 1
  fi
  if [[ "$value" == *$'\n'* || "$value" == *$'\r'* ]]; then
    msg_error "${name} must not contain newlines"
    return 1
  fi
  if [[ "$value" =~ ^[[:space:]] || "$value" =~ [[:space:]]$ ]]; then
    msg_error "${name} must not have leading or trailing whitespace"
    return 1
  fi
}

# Percent-encode a PostgreSQL password for use in DATABASE_URL. The generated
# password is alphanumeric, but an inherited PG_DB_PASS may not be.
_wgswp_urlencode() {
  local LC_ALL=C
  local input="$1"
  local output=""
  local character encoded
  local index

  for ((index = 0; index < ${#input}; index++)); do
    character="${input:index:1}"
    case "$character" in
    [A-Za-z0-9.~_-]) output+="$character" ;;
    *)
      printf -v encoded '%%%02X' "'$character"
      output+="$encoded"
      ;;
    esac
  done
  printf '%s' "$output"
}

# ------------------------------------------------------------------------------
# Framework setup
# ------------------------------------------------------------------------------

setting_up_container
network_check
update_os

if [[ -f /opt/wgswp/wgswp ]]; then
  msg_error "${APP} is already installed. Run /usr/bin/update to update it."
  exit 1
fi

msg_info "Installing Dependencies"
$STD apt install -y ca-certificates curl jq tar
msg_ok "Installed Dependencies"

# ------------------------------------------------------------------------------
# PostgreSQL 18
# ------------------------------------------------------------------------------

PG_VERSION="18" setup_postgresql
PG_DB_NAME="wgswp" PG_DB_USER="wgswp" setup_postgresql_db

# ------------------------------------------------------------------------------
# Service account and runtime directories
# ------------------------------------------------------------------------------

msg_info "Creating Service Account"
if ! id -u wgswp >/dev/null 2>&1; then
  adduser --system --group --home /var/lib/wgswp --shell /usr/sbin/nologin wgswp
fi
install -d -m 0750 -o wgswp -g wgswp /var/lib/wgswp
install -d -m 0755 /opt/wgswp
ln -sfn /var/lib/wgswp /opt/wgswp/data
chown -h wgswp:wgswp /opt/wgswp/data
install -d -m 0700 /var/backups/wgswp
msg_ok "Created Service Account"

# ------------------------------------------------------------------------------
# Application release
# ------------------------------------------------------------------------------

msg_info "Downloading ${APP}"
if ! CLEAN_INSTALL=1 CLEAN_INSTALL_KEEP="data" \
  fetch_and_deploy_gh_release "wgswp" \
  "Ehs-Colin/what-game-should-we-play" "prebuild" "latest" \
  "/opt/wgswp" "wgswp_*_linux_$(arch_resolve amd64 arm64).tar.gz"; then
  msg_error "No installable ${APP} release was found. Publish a v*.*.* release with amd64 and arm64 assets first."
  exit 1
fi

if ! chown -R wgswp:wgswp /opt/wgswp || ! chown -h wgswp:wgswp /opt/wgswp/data; then
  msg_error "Failed to set ownership on the ${APP} deployment"
  exit 1
fi

if [[ ! -x /opt/wgswp/wgswp ]]; then
  msg_error "${APP} binary is missing or not executable at /opt/wgswp/wgswp"
  exit 1
fi
if [[ ! -f /opt/wgswp/web/public/index.html ]]; then
  msg_error "${APP} web assets are missing at /opt/wgswp/web/public"
  exit 1
fi
msg_ok "Downloaded ${APP}"

# ------------------------------------------------------------------------------
# First-run settings
# ------------------------------------------------------------------------------

steam_api_key="${var_steam_api_key:-}"
if [[ -z "$steam_api_key" && -t 0 ]]; then
  read -rsp "Steam API key: " steam_api_key || true
  echo
fi
if [[ -z "$steam_api_key" ]]; then
  msg_error "A Steam API key is required. Create one at https://steamcommunity.com/dev/apikey"
  exit 1
fi
if [[ ! "$steam_api_key" =~ ^[0-9A-Fa-f]{32}$ ]]; then
  msg_warn "The Steam API key is not 32 hexadecimal characters; continuing anyway"
fi

admin_username="${var_admin_username:-}"
if [[ -z "$admin_username" && -t 0 ]]; then
  read -rp "Admin username [admin]: " admin_username || true
fi
admin_username="${admin_username:-admin}"

while ! _wgswp_validate_username "$admin_username"; do
  msg_warn "Admin username must be 3-64 characters, start with a letter or number, and contain only letters, numbers, '.', '_' or '-'"
  if [[ ! -t 0 ]]; then
    msg_error "Invalid admin username: ${admin_username}"
    exit 1
  fi
  read -rp "Admin username [admin]: " admin_username || {
    msg_error "Unable to read an admin username"
    exit 1
  }
  admin_username="${admin_username:-admin}"
done

generated_admin_password=0
admin_password="${var_admin_password:-}"
if [[ -z "$admin_password" ]]; then
  admin_password="$(random_password 24)"
  generated_admin_password=1
elif ((${#admin_password} < 12 || ${#admin_password} > 128)); then
  msg_error "The supplied admin password must be between 12 and 128 characters"
  exit 1
fi

_wgswp_validate_env_value "STEAM_API_KEY" "$steam_api_key" || exit 1
_wgswp_validate_env_value "DEFAULT_ADMIN_USERNAME" "$admin_username" || exit 1
_wgswp_validate_env_value "DEFAULT_ADMIN_PASSWORD" "$admin_password" || exit 1

LOCAL_IP="${LOCAL_IP:-$(hostname -I 2>/dev/null | awk '{print $1}')}"
if [[ -z "$LOCAL_IP" ]]; then
  msg_error "Could not determine the container IP address for APP_ORIGIN"
  exit 1
fi
if [[ "$LOCAL_IP" == *:* ]]; then
  app_origin="http://[${LOCAL_IP}]:3030"
else
  app_origin="http://${LOCAL_IP}:3030"
fi

# ------------------------------------------------------------------------------
# Environment file
# ------------------------------------------------------------------------------

msg_info "Writing Environment File"
install -d -m 0700 /etc/wgswp
(
  umask 077
  cat >/etc/wgswp/wgswp.env <<EOF
PORT=3030
APP_ORIGIN="${app_origin}"
TRUSTED_PROXY_HOPS=0

DATABASE_URL="postgres://wgswp:$(_wgswp_urlencode "$PG_DB_PASS")@127.0.0.1:5432/wgswp?sslmode=disable"
DATABASE_MAX_OPEN_CONNS=10

STEAM_API_KEY="${steam_api_key}"
DEFAULT_ADMIN_USERNAME="${admin_username}"
DEFAULT_ADMIN_PASSWORD="${admin_password}"
EOF
)
chown root:root /etc/wgswp/wgswp.env
chmod 600 /etc/wgswp/wgswp.env
msg_ok "Wrote Environment File"

# ------------------------------------------------------------------------------
# systemd unit
# ------------------------------------------------------------------------------

msg_info "Creating Service"
cat >/etc/systemd/system/wgswp.service <<'EOF'
[Unit]
Description=Game Picker Utility (WGSWP)
Wants=network-online.target
After=network-online.target postgresql.service
Requires=postgresql.service

[Service]
Type=simple
User=wgswp
Group=wgswp
WorkingDirectory=/opt/wgswp
EnvironmentFile=/etc/wgswp/wgswp.env
ExecStart=/opt/wgswp/wgswp
Restart=on-failure
RestartSec=5
UMask=0077
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/wgswp

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable -q --now wgswp
msg_ok "Created Service"

msg_info "Waiting for ${APP} to become healthy"
healthy=0
for _ in $(seq 1 30); do
  if curl -fsS --max-time 5 http://127.0.0.1:3030/health >/dev/null 2>&1; then
    healthy=1
    break
  fi
  sleep 2
done
if [[ "$healthy" -ne 1 ]]; then
  systemctl status wgswp --no-pager || true
  journalctl -u wgswp -n 50 --no-pager || true
  msg_error "${APP} did not become healthy"
  exit 1
fi
msg_ok "${APP} is healthy"

# ------------------------------------------------------------------------------
# Log rotation and credentials
# ------------------------------------------------------------------------------

msg_info "Configuring Log Rotation"
cat >/etc/logrotate.d/wgswp <<'EOF'
/var/lib/wgswp/wgswp.log {
    weekly
    rotate 4
    compress
    missingok
    notifempty
    copytruncate
    su wgswp wgswp
}
EOF
msg_ok "Configured Log Rotation"

msg_info "Saving Credentials"
(
  umask 077
  {
    echo ""
    echo "WGSWP Admin Username: ${admin_username}"
    echo "WGSWP Admin Password: ${admin_password}"
    echo "WGSWP URL: ${app_origin}"
    echo "Steam API Key: ${steam_api_key}"
  } >>/root/wgswp.creds
)
chown root:root /root/wgswp.creds
chmod 600 /root/wgswp.creds
msg_ok "Saved Credentials"

motd_ssh
customize
cleanup_lxc

msg_ok "Completed successfully!\n"
echo -e "${INFO}${YW}Access it using the following URL:${CL}"
echo -e "${GATEWAY}${BGN}${app_origin}${CL}"
echo -e "${INFO}${YW}Credentials are stored in:${CL} /root/wgswp.creds"
if [[ "$generated_admin_password" -eq 1 ]]; then
  echo -e "${INFO}${YW}Generated admin password:${CL} ${BGN}${admin_password}${CL}"
fi
echo -e "${INFO}${YW}The first browser login must create the real account; the default account is then disabled.${CL}"
