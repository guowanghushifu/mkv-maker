#!/usr/bin/env bash
set -euo pipefail

log() {
  printf '[entrypoint] %s\n' "$*"
}

app_data_dir="${APP_DATA_DIR:-/app/data}"
makemkv_config_dir="${MAKEMKV_CONFIG_DIR:-${app_data_dir}/makemkv}"
settings_file="${MAKEMKV_SETTINGS_FILE:-${makemkv_config_dir}/settings.conf}"
default_settings_file="${MAKEMKV_DEFAULT_SETTINGS_FILE:-/etc/makemkv/settings.conf}"
home_dir="${HOME:-/root}"

mkdir -p "$makemkv_config_dir" "$makemkv_config_dir/data" "$app_data_dir"
mkdir -p "$home_dir"

if [[ ! -f "$settings_file" ]]; then
  cp "$default_settings_file" "$settings_file"
  log "initialized MakeMKV settings at $settings_file"
fi

if [[ -L "${home_dir}/.MakeMKV" || ! -e "${home_dir}/.MakeMKV" ]]; then
  ln -sfn "$makemkv_config_dir" "${home_dir}/.MakeMKV"
else
  log "keeping existing ${home_dir}/.MakeMKV"
fi

export MAKEMKV_CONFIG_DIR="$makemkv_config_dir"
export MAKEMKV_SETTINGS_FILE="$settings_file"

if [[ "${MAKEMKV_KEY+x}" == "x" && -n "${MAKEMKV_KEY}" ]]; then
  if [[ "${MAKEMKV_KEY}" == "BETA" ]]; then
    if /usr/local/bin/makemkv-update-beta-key.sh; then
      log "beta key refresh succeeded"
    else
      log "beta key refresh failed, continuing without stopping startup"
    fi
  elif /usr/local/bin/makemkv-set-key.sh "${MAKEMKV_KEY}"; then
    log "custom MakeMKV key set from MAKEMKV_KEY"
  else
    log "custom key update failed, continuing without stopping startup"
  fi
else
  log "MAKEMKV_KEY unset or empty, leaving key unchanged"
fi

exec "$@"
