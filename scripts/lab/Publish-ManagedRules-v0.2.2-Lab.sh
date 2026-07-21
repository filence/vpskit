#!/usr/bin/env bash
set -euo pipefail

/usr/local/bin/vpskit subscription publish
/usr/local/bin/vpskit subscription status

python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text())
publication = json.loads(Path('/var/lib/vpskit/subscription-state.json').read_text())
assert state['config_revision'] == 8, state
assert state['rules']['source_mode'] == 'managed', state
assert publication['status'] == 'COMMITTED', publication
assert publication['client_revision'] == state['config_revision'], publication
assert publication['ruleset_revision'] == state['rules']['revision'], publication
print(f'MANAGED_RULE_PUBLICATION=PASS client_revision={publication["client_revision"]} ruleset_revision={publication["ruleset_revision"]}')
PY
