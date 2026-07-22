#!/usr/bin/env bash
set -euo pipefail

result="$(systemctl show --property=Result --value vpskit-port-hop-subscription-readback.service)"
active="$(systemctl is-active vpskit-port-hop-subscription-readback.service || true)"
printf 'ACTIVE=%s\nRESULT=%s\n' "$active" "$result"
if [ "$result" = 'success' ] && [ "$active" = 'inactive' ]; then
  python3 - <<'PY'
import json
from pathlib import Path

value = json.loads(Path('/root/vpskit-port-hop-subscription-lifecycle.json').read_text())
assert value['status'] == 'PASS', value
assert value['detail']['readback'] == 'PASS', value
PY
  printf '%s\n' 'VPSKIT_PORTHOP_SUBSCRIPTION_READBACK=PASS'
fi
