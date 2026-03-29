#!/usr/bin/env bash
set -euo pipefail

IMAGE_TAG="${IMAGE_TAG:-mkv-remux-web:local}"
NO_CACHE="${NO_CACHE:-0}"

build_args=()
if [[ "${NO_CACHE}" == "1" ]]; then
  build_args+=(--no-cache)
fi

docker build "${build_args[@]}" -t "${IMAGE_TAG}" .
echo "Built image: ${IMAGE_TAG}"
