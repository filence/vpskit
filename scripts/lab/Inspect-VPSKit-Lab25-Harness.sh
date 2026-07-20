#!/usr/bin/env bash
set -Eeuo pipefail

python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
print('version=' + state['vpskit_version'])
print('profile=' + state['profile'])
print('reality_enabled=' + str(state['reality'].get('enabled', False)).lower())
print('reality_port=' + str(state['reality'].get('listen_port', 0)))
print('hysteria2_enabled=' + str(state['hysteria2'].get('enabled', False)).lower())
print('hysteria2_port=' + str(state['hysteria2'].get('listen_port', 0)))
incomplete = []
for path in Path('/var/lib/vpskit/transactions').glob('TX-*/transaction.json'):
    try:
        record = json.loads(path.read_text(encoding='utf-8'))
    except Exception:
        continue
    if record.get('status') == 'IN_PROGRESS':
        incomplete.append((record.get('transaction_id', path.parent.name), record.get('command', '')))
for transaction_id, command in sorted(incomplete):
    print(f'incomplete={transaction_id} command={command}')
print('incomplete_count=' + str(len(incomplete)))
PY

systemctl is-active vpskit-sing-box.service
ss -ltnH | grep -E ':(443|24443) ' || true
ss -lunH | grep -E ':(443|24443) ' || true
pgrep -af 'Test-VPSKit-Lab25-Phase4|vpskit instance|vpskit-lab25-phase4' || true
