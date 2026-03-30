#!/usr/bin/env bash
set -euo pipefail

IMAGE_TAG="${IMAGE_TAG:-mkv-remux-web:local}"
NO_CACHE="${NO_CACHE:-0}"
PLATFORMS="${PLATFORMS:-}"
PUSH="${PUSH:-0}"

if ! docker buildx version >/dev/null 2>&1; then
  echo "Docker Buildx is required for this script."
  echo "Install/enable Buildx and retry."
  exit 1
fi

if [[ "${PUSH}" == "1" ]]; then
  is_registry_qualified=0
  if [[ "${IMAGE_TAG}" == */* ]]; then
    image_namespace="${IMAGE_TAG%%/*}"
    if [[ "${image_namespace}" == "localhost" || "${image_namespace}" == *.* || "${image_namespace}" == *:* ]]; then
      is_registry_qualified=1
    fi
  fi

  if [[ "${IMAGE_TAG}" == "mkv-remux-web:local" || "${is_registry_qualified}" != "1" ]]; then
    echo "PUSH=1 requires a registry-qualified IMAGE_TAG."
    echo "Example: IMAGE_TAG=ghcr.io/<owner>/mkv-remux-web:test"
    exit 1
  fi
fi

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
if [[ "${PUSH}" == "1" ]]; then
  echo "Pushed image: ${IMAGE_TAG}"
else
  echo "Built image: ${IMAGE_TAG}"
fi
