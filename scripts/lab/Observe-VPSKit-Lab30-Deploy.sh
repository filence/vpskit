#!/usr/bin/env bash
set -u

if [ -e /var/lib/vpskit/state.json ]; then
    printf 'state_present=yes\n'
    python3 - <<'PY'
import json
from pathlib import Path
state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
print('state_schema=' + str(state.get('schema_version')))
print('vpskit_version=' + str(state.get('vpskit_version')))
PY
else
    printf 'state_present=no\n'
fi
printf 'xray_active=%s\n' "$(systemctl is-active vpskit-xray.service 2>/dev/null || true)"
printf 'sing_box_active=%s\n' "$(systemctl is-active vpskit-sing-box.service 2>/dev/null || true)"
ps -eo pid=,ppid=,etime=,comm= | awk '$4 == "bash" || $4 == "lego" || $4 == "vpskit" {print}'
