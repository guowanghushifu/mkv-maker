#!/usr/bin/env bash
set -euo pipefail

IMAGE_TAG="${IMAGE_TAG:-mkv-remux-web:local}"
NO_CACHE="${NO_CACHE:-0}"
PLATFORMS="${PLATFORMS:-}"
PUSH="${PUSH:-0}"

build_args=(buildx build)

if [[ "${NO_CACHE}" == "1" ]]; then
  build_args+=(--no-cache)
fi

if [[ -n "${PLATFORMS}" ]]; then
  build_args+=(--platform "${PLATFORMS}")
fi

if [[ "${PUSH}" == "1" ]]; then
  build_args+=(--push)
elif [[ -z "${PLATFORMS}" || "${PLATFORMS}" != *","* ]]; then
  build_args+=(--load)
else
  echo "Multi-arch builds cannot be loaded into the local Docker daemon."
  echo "Set PUSH=1 to publish, or provide a single platform."
  exit 1
fi

build_args+=(-t "${IMAGE_TAG}" .)

docker "${build_args[@]}"
echo "Built image: ${IMAGE_TAG}"
