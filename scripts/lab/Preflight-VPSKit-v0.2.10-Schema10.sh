#!/usr/bin/env bash
set -euo pipefail
umask 077

readonly expected_version='v0.2.9-lab.1'
readonly minimum_available_bytes=268435456
readonly state_path='/var/lib/vpskit/state.json'
readonly publication_path='/var/lib/vpskit/subscription-state.json'

test -x /usr/local/bin/vpskit
test -f "$state_path"
test -f "$publication_path"
test "$(/usr/local/bin/vpskit version)" = "vpskit ${expected_version}"
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service

available_bytes="$(df --output=avail -B1 /var/lib/vpskit | tail -n 1 | tr -d ' ')"
test "$available_bytes" -ge "$minimum_available_bytes"

plan="$(/usr/local/bin/vpskit migrate plan)"
python3 - "$plan" <<'PY'
import json
import sys

result = json.loads(sys.argv[1])
assert result['status'] == 'PASS', result
detail = result['detail']
assert detail['source_schema'] == 9, detail
assert detail['target_schema'] == 9, detail
assert detail['migration_required'] is False, detail
PY

printf 'SCHEMA10_PREFLIGHT=PASS current=%s available_bytes=%s state_sha256=%s publication_sha256=%s\n' \
  "$expected_version" \
  "$available_bytes" \
  "$(sha256sum "$state_path" | awk '{print $1}')" \
  "$(sha256sum "$publication_path" | awk '{print $1}')"
