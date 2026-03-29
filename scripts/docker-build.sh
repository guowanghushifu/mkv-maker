#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"

image_name="${IMAGE_NAME:-mkv-maker:local}"
makemkv_version="${MAKEMKV_VERSION:-1.18.1}"

docker build \
  "$@" \
  -f "${repo_root}/Dockerfile" \
  -t "${image_name}" \
  --build-arg "MAKEMKV_VERSION=${makemkv_version}" \
  "${repo_root}"
