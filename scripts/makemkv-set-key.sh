#!/usr/bin/env bash
set -euo pipefail

settings_file="${MAKEMKV_SETTINGS_FILE:-${MAKEMKV_CONFIG_DIR:-/app/data/makemkv}/settings.conf}"
makemkv_key="${1:-${MAKEMKV_KEY:-}}"

if [[ -z "$makemkv_key" ]]; then
  echo "[makemkv-set-key] key is empty" >&2
  exit 1
fi

mkdir -p "$(dirname "$settings_file")"
touch "$settings_file"

escaped_key="${makemkv_key//\\/\\\\}"
escaped_key="${escaped_key//&/\\&}"
escaped_key="${escaped_key//\"/\\\"}"
key_line="app_Key = \"${escaped_key}\""

if grep -Eq '^[[:space:]]*app_Key[[:space:]]*=' "$settings_file"; then
  sed -i -E "s|^[[:space:]]*app_Key[[:space:]]*=.*$|${key_line}|" "$settings_file"
else
  printf '\n%s\n' "$key_line" >> "$settings_file"
fi

echo "[makemkv-set-key] key stored in $settings_file"
