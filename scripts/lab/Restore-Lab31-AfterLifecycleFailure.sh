#!/usr/bin/env bash
set -euo pipefail

backup_id='BK-20260720-011151-08ba86'
test -f "/var/lib/vpskit/backups/$backup_id/backup.json"
/usr/local/bin/vpskit restore "$backup_id" --yes >/dev/null
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.31'
python3 <<'PY'
import json
with open('/var/lib/vpskit/state.json', encoding='utf-8') as stream:
    state = json.load(stream)
assert state['profile'] == 'balanced', state
assert state['reality']['enabled'] is True, state
assert state['hysteria2']['enabled'] is True, state
PY
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
/usr/local/bin/vpskit doctor >/dev/null
echo 'LAB31_FAILURE_RECOVERY=PASS'
