#!/bin/sh
set -eu

CONFIG_DIR=/config
CONFIG_FILE=/config/settings.conf
CONFIG_LINK=/config/.MakeMKV
DEFAULTS_FILE=/defaults/settings.conf
CRON_LOG=/var/log/makemkv-beta-key.log

mkdir -p "$CONFIG_DIR" /config/data

if [ ! -s "$CONFIG_FILE" ]; then
    cp "$DEFAULTS_FILE" "$CONFIG_FILE"
fi

if [ -L "$CONFIG_LINK" ]; then
    if [ "$(readlink "$CONFIG_LINK")" != "$CONFIG_DIR" ]; then
        rm -f "$CONFIG_LINK"
        ln -s "$CONFIG_DIR" "$CONFIG_LINK"
    fi
elif [ -e "$CONFIG_LINK" ]; then
    rm -rf "$CONFIG_LINK"
    ln -s "$CONFIG_DIR" "$CONFIG_LINK"
else
    ln -s "$CONFIG_DIR" "$CONFIG_LINK"
fi

touch "$CRON_LOG"

echo "Refreshing MakeMKV beta key..."
if ! /opt/makemkv/bin/makemkv-update-beta-key "$CONFIG_FILE"; then
    echo "WARNING: failed to refresh MakeMKV beta key at startup" >&2
fi

cron
exec /app/server
