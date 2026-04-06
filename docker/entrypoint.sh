#!/bin/sh
set -eu

CONFIG_DIR=/config
CONFIG_FILE=/config/settings.conf
DEFAULTS_FILE=/defaults/settings.conf
CRON_LOG=/var/log/makemkv-beta-key.log

mkdir -p "$CONFIG_DIR" /config/data

if [ ! -s "$CONFIG_FILE" ]; then
    cp "$DEFAULTS_FILE" "$CONFIG_FILE"
fi

touch "$CRON_LOG"

echo "Refreshing MakeMKV beta key..."
if ! /opt/makemkv/bin/makemkv-update-beta-key "$CONFIG_FILE"; then
    echo "WARNING: failed to refresh MakeMKV beta key at startup" >&2
fi

cron
exec /app/server
