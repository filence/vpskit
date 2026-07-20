#!/usr/bin/env bash
set -Eeuo pipefail

python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
print('version=' + state['vpskit_version'])
print('profile=' + state['profile'])
print('reality_enabled=' + str(state['reality'].get('enabled', False)).lower())
print('hysteria2_enabled=' + str(state['hysteria2'].get('enabled', False)).lower())
for manifest_path in sorted(Path('/var/lib/vpskit/backups').glob('BK-*/backup.json')):
    try:
        manifest = json.loads(manifest_path.read_text(encoding='utf-8'))
        backup_state = json.loads((manifest_path.parent / 'state.json').read_text(encoding='utf-8'))
    except Exception:
        continue
    print('backup=' + manifest_path.parent.name + ' profile=' + backup_state.get('profile', '') + ' version=' + backup_state.get('vpskit_version', ''))
PY

systemctl is-active vpskit-sing-box.service
/usr/local/bin/vpskit doctor >/dev/null
printf 'REALITY_RECOVERY_INSPECT=PASS\n'
