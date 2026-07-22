#!/usr/bin/env bash
set -euo pipefail

python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
assert state['hysteria2']['obfuscation'] == 'salamander', state['hysteria2']
assert state['config_revision'] >= 15, state
print('SALAMANDER_STATE=PASS')
PY
/usr/local/bin/vpskit status >/root/vpskit-salamander-status.json
/usr/local/bin/vpskit subscription status >/root/vpskit-salamander-subscription-status.json
python3 - <<'PY'
import json
from pathlib import Path

status = json.loads(Path('/root/vpskit-salamander-status.json').read_text(encoding='utf-8'))
assert status['status'] == 'PASS', status
subscription = json.loads(Path('/root/vpskit-salamander-subscription-status.json').read_text(encoding='utf-8'))
assert subscription['status'] == 'PASS', subscription
print('SALAMANDER_RUNTIME_AND_SUBSCRIPTION=PASS')
PY
/usr/local/bin/vpskit hysteria2 inspect >/root/vpskit-salamander-inspect.json
python3 - <<'PY'
import json
from pathlib import Path

config = json.loads(Path('/etc/vpskit/generated/sing-box.json').read_text(encoding='utf-8'))
inbound = next(item for item in config['inbounds'] if item['type'] == 'hysteria2')
assert inbound['obfs']['type'] == 'salamander', inbound
inspect = json.loads(Path('/root/vpskit-salamander-inspect.json').read_text(encoding='utf-8'))
assert inspect['detail']['obfuscation'] == {'type': 'salamander', 'enabled': True}, inspect
print('SALAMANDER_RENDERED_CONFIG=PASS')
PY
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
ss -lunH | grep -Eq '(:|\[::\]:)443\b'
/usr/local/lib/vpskit/bin/sing-box check -c /etc/vpskit/generated/sing-box.json
/usr/local/bin/vpskit doctor >/root/vpskit-salamander-doctor.json
python3 - <<'PY'
import json
from pathlib import Path

doctor = json.loads(Path('/root/vpskit-salamander-doctor.json').read_text(encoding='utf-8'))
assert doctor['status'] == 'PASS', doctor
print('SALAMANDER_DOCTOR=PASS')
PY
