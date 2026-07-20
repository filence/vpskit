#!/usr/bin/env bash
set -u

check() {
  local name="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    printf '%s=PASS\n' "$name"
  else
    printf '%s=FAIL\n' "$name"
  fi
}

check version test "$(/usr/local/bin/vpskit version 2>/dev/null)" = 'vpskit v0.1.0-lab.29'
check sing_box_active systemctl is-active --quiet vpskit-sing-box.service
check xray_inactive bash -c '! systemctl is-active --quiet vpskit-xray.service'
schema="$(python3 -c 'import json; print(json.load(open("/var/lib/vpskit/state.json"))["schema_version"])')"
check schema3 test "$schema" = 3
check cloudflare_env_readable test -r /var/lib/vpskit/secrets/cloudflare.env
check acme_env_readable test -r /var/lib/vpskit/secrets/acme.env
check tcp443 bash -c "ss -ltnH | grep -q ':443 '"
check udp443 bash -c "ss -lunH | grep -q ':443 '"
check compat_unit_absent bash -c "! systemctl list-unit-files --no-legend | grep -q '^vpskit-xray-compat\\.service'"
check compat_runtime_absent test ! -e /root/vpskit-xray-compat-runtime
check compat_upload_absent test ! -e /root/vpskit-xray-compat-xray
printf 'cloudflare_mode=%s\n' "$(stat -c '%a' /var/lib/vpskit/secrets/cloudflare.env 2>/dev/null || printf absent)"
printf 'acme_mode=%s\n' "$(stat -c '%a' /var/lib/vpskit/secrets/acme.env 2>/dev/null || printf absent)"
