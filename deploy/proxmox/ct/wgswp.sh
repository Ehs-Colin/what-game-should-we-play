#!/usr/bin/env bash
# Copyright (c) 2026 Ehs-Colin
# License: MIT
# Source: https://github.com/Ehs-Colin/what-game-should-we-play
#
# Proxmox VE LXC creator for Game Picker Utility (WGSWP).
# The application scripts live in deploy/proxmox; the engine lives in
# community-scripts/core.

export COMMUNITY_SCRIPTS_URL="${COMMUNITY_SCRIPTS_URL:-https://raw.githubusercontent.com/Ehs-Colin/what-game-should-we-play/main/deploy/proxmox}"

_cs_boot="${COMMUNITY_SCRIPTS_CORE_DIR:-$(dirname "${BASH_SOURCE[0]}")/../../core}/core/build.func"
# shellcheck disable=SC1090,SC1091
source "$_cs_boot" 2>/dev/null || source <(curl -fsSL "${COMMUNITY_SCRIPTS_CORE_URL:-https://raw.githubusercontent.com/community-scripts/core/main}/core/build.func")

APP="WGSWP"
var_tags="${var_tags:-gaming;steam}"
var_cpu="${var_cpu:-1}"
var_ram="${var_ram:-1024}"
var_disk="${var_disk:-8}"
var_os="${var_os:-debian}"
var_version="${var_version:-13}"
var_arm64="${var_arm64:-yes}"
var_unprivileged="${var_unprivileged:-1}"

var_steam_api_key="${var_steam_api_key:-}"
var_admin_username="${var_admin_username:-}"
var_admin_password="${var_admin_password:-}"
export var_steam_api_key var_admin_username var_admin_password

# Set by set_std_mode() before update_deb_based runs; declare it here so the
# script is shellcheck-clean when sourced outside the engine.
STD="${STD:-}"

header_info "$APP"
variables
color
catch_errors

# ------------------------------------------------------------------------------
# Update helpers
# ------------------------------------------------------------------------------

_wgswp_wait_healthy() {
  local attempt
  for ((attempt = 1; attempt <= 30; attempt++)); do
    if curl -fsS --max-time 5 http://127.0.0.1:3030/health >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  return 1
}

_wgswp_show_failure_logs() {
  systemctl status wgswp --no-pager || true
  journalctl -u wgswp -n 50 --no-pager || true
}

# Restore the binary and web assets saved before the failed deployment. The
# database is deliberately not restored automatically: migrations are
# forward-only and the operator must decide whether the dump is needed.
_wgswp_rollback() {
  local rollback_dir="$1"
  local previous_version="$2"

  msg_info "Rolling Back ${APP}"
  systemctl stop wgswp >/dev/null 2>&1 || true

  mkdir -p /opt/wgswp || return 1
  rm -rf /opt/wgswp/wgswp /opt/wgswp/web || return 1
  cp -a "${rollback_dir}/wgswp" /opt/wgswp/wgswp || return 1
  cp -a "${rollback_dir}/web" /opt/wgswp/web || return 1
  [[ -L /opt/wgswp/data ]] || ln -sfn /var/lib/wgswp /opt/wgswp/data || return 1
  chown -R wgswp:wgswp /opt/wgswp || return 1
  chown -h wgswp:wgswp /opt/wgswp/data || return 1

  if [[ -n "$previous_version" ]]; then
    printf '%s\n' "$previous_version" >"$HOME/.wgswp" || return 1
  else
    rm -f "$HOME/.wgswp" || return 1
  fi

  if systemctl start wgswp; then
    if _wgswp_wait_healthy; then
      msg_ok "Restored the previous ${APP} release"
    else
      _wgswp_show_failure_logs
      msg_warn "Previous ${APP} release restored but did not become healthy"
    fi
  else
    msg_error "Failed to start the previous ${APP} release"
    return 1
  fi

  return 0
}

_wgswp_fail_update() {
  local reason="$1"
  local rollback_dir="$2"
  local previous_version="$3"
  local dump="$4"

  if _wgswp_rollback "$rollback_dir" "$previous_version"; then
    msg_error "${reason} Restored the previous ${APP} release. Database dump: ${dump}"
  else
    msg_error "${reason} Automatic rollback failed. Database dump: ${dump}"
  fi

  msg_warn "Migrations are forward-only. If the failed release changed the schema, restore the dump with pg_restore before starting the previous binary."
  return 0
}

# ------------------------------------------------------------------------------
# Debian update path
# ------------------------------------------------------------------------------

update_deb_based() {
  if [[ ! -f /opt/wgswp/wgswp ]]; then
    msg_error "No ${APP} Installation Found!"
    exit 1
  fi

  msg_info "Updating Debian Packages"
  # shellcheck disable=SC2086
  $STD apt-get update
  # shellcheck disable=SC2086
  $STD apt-get -y upgrade
  msg_ok "Updated Debian Packages"

  # Keep PostgreSQL on major version 18. A major-version upgrade is a separate
  # migration and must not be triggered implicitly by an application update.
  PG_VERSION="18" setup_postgresql

  if ! check_for_gh_release "wgswp" "Ehs-Colin/what-game-should-we-play"; then
    # A package upgrade may have restarted PostgreSQL. Rebuild the app's
    # connection pool so it does not keep using dead connections.
    if systemctl is-active --quiet wgswp; then
      msg_info "Restarting ${APP} after package updates"
      if ! systemctl restart wgswp; then
        msg_error "Failed to restart ${APP}"
        return 1
      fi
      if ! _wgswp_wait_healthy; then
        _wgswp_show_failure_logs
        msg_error "${APP} did not become healthy after restart"
        return 1
      fi
      msg_ok "Restarted ${APP}"
    else
      msg_warn "${APP} is not running; skipped post-update restart"
    fi
    msg_ok "Updated successfully!"
    return 0
  fi

  local rollback_dir="/var/backups/wgswp/rollback"
  local previous_version=""
  local dump=""
  local pg_bin=""

  msg_info "Backing Up Database"
  install -d -m 0700 /var/backups/wgswp
  pg_bin="/usr/lib/postgresql/$(find /usr/lib/postgresql -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | sort -V | tail -n 1)/bin"
  dump="/var/backups/wgswp/wgswp-$(date +%Y%m%d-%H%M%S).dump"
  # pg_dump runs as postgres, but the 0700 backup directory is root-owned, so
  # root opens the redirection and owns the resulting file.
  if ! runuser -u postgres -- "$pg_bin/pg_dump" --format=custom --no-owner wgswp >"$dump"; then
    rm -f "$dump"
    msg_error "Database backup failed; aborting update"
    return 1
  fi
  chmod 600 "$dump"
  # Keep only the three newest dumps.
  find /var/backups/wgswp -maxdepth 1 -type f -name 'wgswp-*.dump' -printf '%T@ %p\n' |
    sort -nr | tail -n +4 | cut -d' ' -f2- | xargs -r rm -f -- || true
  msg_ok "Backed Up Database"

  msg_info "Saving Rollback Copy"
  rm -rf "$rollback_dir"
  mkdir -p "$rollback_dir"
  cp -a /opt/wgswp/wgswp "$rollback_dir/wgswp"
  cp -a /opt/wgswp/web "$rollback_dir/web"
  previous_version="$(cat "$HOME/.wgswp" 2>/dev/null || true)"
  msg_ok "Saved Rollback Copy"

  msg_info "Stopping Service"
  systemctl stop wgswp
  msg_ok "Stopped Service"

  msg_info "Deploying ${APP}"
  if ! CLEAN_INSTALL=1 CLEAN_INSTALL_KEEP="data" \
    fetch_and_deploy_gh_release "wgswp" \
    "Ehs-Colin/what-game-should-we-play" "prebuild" "latest" \
    "/opt/wgswp" "wgswp_*_linux_$(arch_resolve amd64 arm64).tar.gz"; then
    _wgswp_fail_update "Failed to deploy ${APP}." "$rollback_dir" "$previous_version" "$dump"
    return 1
  fi

  if ! chown -R wgswp:wgswp /opt/wgswp || ! chown -h wgswp:wgswp /opt/wgswp/data; then
    _wgswp_fail_update "Failed to set ownership on the ${APP} deployment." "$rollback_dir" "$previous_version" "$dump"
    return 1
  fi
  msg_ok "Deployed ${APP}"

  if [[ ! -x /opt/wgswp/wgswp || ! -f /opt/wgswp/web/public/index.html || ! -f /etc/wgswp/wgswp.env || ! -d /var/lib/wgswp || ! -L /opt/wgswp/data ]]; then
    _wgswp_fail_update "Deployed ${APP} is incomplete or lost its data/config paths." "$rollback_dir" "$previous_version" "$dump"
    return 1
  fi

  msg_info "Starting Service"
  if ! systemctl start wgswp; then
    _wgswp_show_failure_logs
    _wgswp_fail_update "Failed to start updated ${APP}." "$rollback_dir" "$previous_version" "$dump"
    return 1
  fi
  msg_ok "Started Service"

  if ! _wgswp_wait_healthy || ! systemctl is-active --quiet wgswp; then
    _wgswp_show_failure_logs
    _wgswp_fail_update "Updated ${APP} failed its health check." "$rollback_dir" "$previous_version" "$dump"
    return 1
  fi

  msg_ok "Updated successfully!"
}

function update_script() {
  header_info
  check_container_storage
  check_container_resources
  if [[ ! -f /opt/wgswp/wgswp ]]; then
    msg_error "No ${APP} Installation Found!"
    exit 1
  fi
  run_os_update
  exit
}

start
build_container
description

msg_ok "Completed successfully!\n"
echo -e "${CREATING}${GN}${APP} setup has been successfully initialized!${CL}"
echo -e "${INFO}${YW}Access it using the following URL:${CL}"
echo -e "${GATEWAY}${BGN}http://${IP}:3030${CL}"
