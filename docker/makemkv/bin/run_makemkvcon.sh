#!/bin/sh
set -eu

export HOME=/config

if [ -n "${FAKETIME:-}" ]; then
    export FAKETIME
    export LD_PRELOAD=/usr/local/lib/libfaketime.so.1
    export FAKETIME_DONT_FAKE_MONOTONIC=1
fi

exec /opt/makemkv/bin/makemkvcon "$@"
