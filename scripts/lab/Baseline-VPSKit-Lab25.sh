#!/usr/bin/env bash
set -Eeuo pipefail

test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.24'
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-certificate-renew.timer
/usr/local/lib/vpskit/bin/sing-box check -c /etc/vpskit/generated/sing-box.json

python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
assert state['schema_version'] == 2, state['schema_version']
assert state['profile'] == 'balanced', state['profile']
assert state['reality']['listen_port'] == 443
assert state['hysteria2']['listen_port'] == 443
for path in (
    '/etc/vpskit/exports/mihomo.yaml',
    '/etc/vpskit/exports/sing-box-reality.json',
    '/etc/vpskit/exports/sing-box-hysteria2.json',
    '/etc/vpskit/exports/share-links.txt',
):
    assert Path(path).is_file(), path
print('LAB25_BASELINE_STATE=PASS')
PY

/usr/local/bin/vpskit doctor >/tmp/vpskit-lab25-baseline-doctor.json
python3 - <<'PY'
import json

with open('/tmp/vpskit-lab25-baseline-doctor.json', encoding='utf-8') as handle:
    result = json.load(handle)
assert result['status'] == 'PASS', result
print('LAB25_BASELINE_DOCTOR=PASS')
PY
rm -f -- /tmp/vpskit-lab25-baseline-doctor.json
printf 'LAB25_BASELINE=PASS\n'
