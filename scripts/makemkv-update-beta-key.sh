#!/usr/bin/env bash
set -euo pipefail

forum_url="${MAKEMKV_BETA_KEY_URL:-https://forum.makemkv.com/forum/viewtopic.php?f=5&t=1053}"
set_key_script="${MAKEMKV_SET_KEY_SCRIPT:-/usr/local/bin/makemkv-set-key.sh}"

if [[ ! -x "$set_key_script" ]]; then
  echo "[makemkv-update-beta-key] missing executable set-key script: $set_key_script" >&2
  exit 1
fi

forum_page="$(curl -fsSL --retry 3 --retry-delay 2 "$forum_url")"

beta_key="$(printf '%s' "$forum_page" | tr '\n' ' ' | sed -nE 's/.*<code>(T-[A-Za-z0-9-]+)<\/code>.*/\1/p' | head -n1)"
if [[ -z "$beta_key" ]]; then
  beta_key="$(printf '%s' "$forum_page" | grep -Eo 'T-[A-Za-z0-9-]{20,}' | head -n1 || true)"
fi

if [[ -z "$beta_key" ]]; then
  echo "[makemkv-update-beta-key] failed to extract beta key from $forum_url" >&2
  exit 1
fi

"$set_key_script" "$beta_key"
echo "[makemkv-update-beta-key] beta key updated"
