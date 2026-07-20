#!/usr/bin/env bash
set -Eeuo pipefail

test -x /usr/local/bin/vpskit
/usr/local/bin/vpskit version
/usr/local/bin/vpskit doctor >/dev/null
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
print('STATE_SCHEMA=' + str(state['schema_version']))
print('PROFILE=' + state['profile'])
print('REALITY_TARGET=' + state['reality_server_name'])
print('CONFIG_REVISION=' + str(state.get('config_revision', 0)))
print('TCP_PORT=' + str(state['reality']['listen_port']))
print('UDP_PORT=' + str(state['hysteria2']['listen_port']))
PY
df -Pk / /var/lib/vpskit | tail -n +2 | awk '{print "DISK_AVAILABLE_KB=" $4 " MOUNT=" $6}'
echo 'LAB32_PREFLIGHT=PASS'
