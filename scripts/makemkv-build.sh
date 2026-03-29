#!/usr/bin/env bash
set -euo pipefail

log() {
  printf '[makemkv-build] %s\n' "$*"
}

download() {
  local url=$1
  local output=$2
  curl -fsSL --retry 3 --retry-delay 2 "$url" -o "$output"
}

MAKEMKV_VERSION="${MAKEMKV_VERSION:?MAKEMKV_VERSION is required}"
MAKEMKV_PREFIX="${MAKEMKV_PREFIX:-/opt/makemkv}"
BUILD_ROOT="$(mktemp -d /tmp/makemkv-build.XXXXXX)"
JOBS="${MAKEMKV_JOBS:-$(getconf _NPROCESSORS_ONLN 2>/dev/null || echo 1)}"

trap 'rm -rf "$BUILD_ROOT"' EXIT

OSS_TARBALL="$BUILD_ROOT/makemkv-oss.tar.gz"
BIN_TARBALL="$BUILD_ROOT/makemkv-bin.tar.gz"
OSS_URL="https://www.makemkv.com/download/makemkv-oss-${MAKEMKV_VERSION}.tar.gz"
BIN_URL="https://www.makemkv.com/download/makemkv-bin-${MAKEMKV_VERSION}.tar.gz"

log "Downloading MakeMKV OSS source ${MAKEMKV_VERSION}"
download "$OSS_URL" "$OSS_TARBALL"
log "Downloading MakeMKV binary package ${MAKEMKV_VERSION}"
download "$BIN_URL" "$BIN_TARBALL"

tar -xzf "$OSS_TARBALL" -C "$BUILD_ROOT"
tar -xzf "$BIN_TARBALL" -C "$BUILD_ROOT"

OSS_DIR="$BUILD_ROOT/makemkv-oss-${MAKEMKV_VERSION}"
BIN_DIR="$BUILD_ROOT/makemkv-bin-${MAKEMKV_VERSION}"

log "Building makemkv-oss"
(
  cd "$OSS_DIR"
  ./configure --prefix="$MAKEMKV_PREFIX" --disable-gui
  make -j"$JOBS"
  make install
)

log "Building makemkv-bin"
(
  cd "$BIN_DIR"
  mkdir -p tmp
  echo accepted > tmp/eula_accepted
  make -j"$JOBS"
  make install PREFIX="$MAKEMKV_PREFIX"
)

# The web service only needs makemkvcon.
rm -f "$MAKEMKV_PREFIX/bin/makemkv"
rm -rf "$MAKEMKV_PREFIX/share/applications" "$MAKEMKV_PREFIX/share/icons"

log "Installed MakeMKV to $MAKEMKV_PREFIX"
