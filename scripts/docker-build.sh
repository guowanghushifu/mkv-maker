#!/usr/bin/env bash
set -euo pipefail

IMAGE_TAG="${IMAGE_TAG:-mkv-remux-web:local}"

docker build -t "${IMAGE_TAG}" .
echo "Built image: ${IMAGE_TAG}"
