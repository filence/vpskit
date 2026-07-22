#!/usr/bin/env bash
set -euo pipefail

readonly before_revision="$(python3 -c 'import json; print(json.load(open("/var/lib/vpskit/state.json"))["config_revision"])')"

/usr/local/bin/vpskit hysteria2 salamander enable --yes >/root/vpskit-salamander-enable.json
python3 - "$before_revision" <<'PY'
import json
import sys
from pathlib import Path

before = int(sys.argv[1])
result = json.loads(Path('/root/vpskit-salamander-enable.json').read_text(encoding='utf-8'))
detail = result['detail']
assert result['status'] == 'PASS', result
assert detail['result'] == 'ENABLED', detail
assert int(detail['config_revision']) == before + 1, detail
assert detail['client_update_required'] is True, detail
assert detail['manual_validation_required'] is True, detail
state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
assert state['hysteria2']['obfuscation'] == 'salamander', state['hysteria2']
assert state['config_revision'] == before + 1, state
PY

/usr/local/bin/vpskit hysteria2 inspect >/root/vpskit-salamander-inspect.json
python3 - <<'PY'
import json
from pathlib import Path

detail = json.loads(Path('/root/vpskit-salamander-inspect.json').read_text(encoding='utf-8'))['detail']
assert detail['obfuscation'] == {'type': 'salamander', 'enabled': True}, detail
PY
/usr/local/bin/sing-box check -c /etc/vpskit/generated/sing-box.json
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
printf 'SALAMANDER_ENABLE_SERVER_ACCEPTANCE=PASS\n'
