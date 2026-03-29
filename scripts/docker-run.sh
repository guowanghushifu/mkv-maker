#!/usr/bin/env bash
set -euo pipefail

IMAGE_TAG="${IMAGE_TAG:-mkv-remux-web:local}"
APP_DATA_HOST_DIR="${APP_DATA_HOST_DIR:-$PWD/.data}"
BD_INPUT_HOST_DIR="${BD_INPUT_HOST_DIR:-$PWD/bd_input}"
REMUX_OUTPUT_HOST_DIR="${REMUX_OUTPUT_HOST_DIR:-$PWD/remux_output}"

if [[ -z "${APP_PASSWORD:-}" ]]; then
  echo "APP_PASSWORD is required."
  echo "Example: APP_PASSWORD=secret ./scripts/docker-run.sh"
  exit 1
fi

mkdir -p "${APP_DATA_HOST_DIR}" "${REMUX_OUTPUT_HOST_DIR}"

docker run --rm -it \
  -p 8080:8080 \
  -e APP_PASSWORD="${APP_PASSWORD}" \
  -e APP_DATA_DIR=/app/data \
  -e BD_INPUT_DIR=/bd_input \
  -e REMUX_OUTPUT_DIR=/remux \
  -e LISTEN_ADDR=:8080 \
  -v "${APP_DATA_HOST_DIR}:/app/data" \
  -v "${BD_INPUT_HOST_DIR}:/bd_input:ro" \
  -v "${REMUX_OUTPUT_HOST_DIR}:/remux" \
  "${IMAGE_TAG}"
