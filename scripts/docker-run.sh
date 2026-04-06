#!/usr/bin/env bash
set -euo pipefail

IMAGE_TAG="${IMAGE_TAG:-mkv-remux-web:local}"
APP_DATA_HOST_DIR="${APP_DATA_HOST_DIR:-$PWD/.data}"
CONFIG_HOST_DIR="${CONFIG_HOST_DIR:-$PWD/.config}"
BD_INPUT_HOST_DIR="${BD_INPUT_HOST_DIR:-$PWD/bd_input}"
REMUX_OUTPUT_HOST_DIR="${REMUX_OUTPUT_HOST_DIR:-$PWD/remux_output}"

if [[ -z "${APP_PASSWORD:-}" ]]; then
  echo "APP_PASSWORD is required."
  echo "Example: APP_PASSWORD=secret ./scripts/docker-run.sh"
  exit 1
fi

mkdir -p "${APP_DATA_HOST_DIR}" "${CONFIG_HOST_DIR}" "${REMUX_OUTPUT_HOST_DIR}"

docker run --rm -it \
  -p 8080:8080 \
  -e APP_PASSWORD="${APP_PASSWORD}" \
  -v "${APP_DATA_HOST_DIR}:/app/data" \
  -v "${CONFIG_HOST_DIR}:/config" \
  -v "${BD_INPUT_HOST_DIR}:/bd_input:ro" \
  -v "${REMUX_OUTPUT_HOST_DIR}:/remux" \
  "${IMAGE_TAG}"
