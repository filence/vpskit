#!/usr/bin/env bash
set -u

printf 'VERSION=%s\n' "$(/usr/local/bin/vpskit version 2>&1)"
printf 'XRAY='; systemctl is-active vpskit-xray.service 2>&1 || true
printf 'SING_BOX='; systemctl is-active vpskit-sing-box.service 2>&1 || true
printf 'PORTHOP_UNIT='; systemctl is-active vpskit-hysteria2-port-hop.service 2>&1 || true
printf 'PORTHOP_TABLE='; nft list table inet vpskit_hysteria2_port_hop >/dev/null 2>&1 && echo present || echo absent
python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text())
print('PORTHOP_STATE=' + json.dumps(state['hysteria2'].get('port_hopping', {}), sort_keys=True))
print('CONFIG_REVISION=' + str(state['config_revision']))
PY
printf 'LOCK='; test -e /var/lib/vpskit/locks/vpskit.lock && echo present || echo absent
printf 'RECENT_TRANSACTIONS='; find /var/lib/vpskit/transactions -mindepth 2 -maxdepth 2 -name status.json -printf '%p\n' 2>/dev/null | sort | tail -n 3 || true
