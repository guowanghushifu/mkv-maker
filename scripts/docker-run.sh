#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"

image_name="${IMAGE_NAME:-mkv-maker:local}"
host_port="${HOST_PORT:-8080}"
app_password="${APP_PASSWORD:-changeme}"
host_data_dir="${DATA_DIR:-${repo_root}/.data/app}"
host_input_dir="${INPUT_DIR:-${repo_root}/.data/bd_input}"
host_output_dir="${OUTPUT_DIR:-${repo_root}/.data/remux}"

mkdir -p "$host_data_dir" "$host_input_dir" "$host_output_dir"

makemkv_key_env=()
if [[ "${MAKEMKV_KEY+x}" == "x" ]]; then
  makemkv_key_env=(-e "MAKEMKV_KEY=${MAKEMKV_KEY}")
fi

docker run --rm -it \
  -p "${host_port}:8080" \
  -e "APP_PASSWORD=${app_password}" \
  -e "APP_DATA_DIR=/app/data" \
  -e "BD_INPUT_DIR=/bd_input" \
  -e "REMUX_OUTPUT_DIR=/remux" \
  "${makemkv_key_env[@]}" \
  -v "${host_data_dir}:/app/data" \
  -v "${host_input_dir}:/bd_input" \
  -v "${host_output_dir}:/remux" \
  "${image_name}" \
  "$@"
