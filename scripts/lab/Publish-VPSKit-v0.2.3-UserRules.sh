#!/usr/bin/env bash
set -euo pipefail

/usr/local/bin/vpskit subscription publish
/usr/local/bin/vpskit subscription status
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service

python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text())
publication = json.loads(Path('/var/lib/vpskit/subscription-state.json').read_text())
assert state['schema_version'] == 9, state
assert state['rules'].get('user_rules', []) == [], state
assert publication['status'] == 'COMMITTED', publication
assert publication['client_revision'] == state['config_revision'], publication
print(f'USER_RULES_PUBLICATION=PASS config_revision={state["config_revision"]}')
PY
