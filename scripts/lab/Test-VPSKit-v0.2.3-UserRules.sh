#!/usr/bin/env bash
set -euo pipefail

test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.2.3-lab.1'

/usr/local/bin/vpskit rules whitelist add --domain whitelist-test.invalid --yes
/usr/local/bin/vpskit rules whitelist list
/usr/local/bin/vpskit rules custom add --type domain-suffix --value custom-test.invalid --policy proxy --yes
/usr/local/bin/vpskit rules custom check
/usr/local/bin/vpskit rules custom remove --type domain-suffix --value custom-test.invalid --policy proxy --yes
/usr/local/bin/vpskit rules whitelist remove --domain whitelist-test.invalid --yes
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
assert state['config_revision'] > 8, state
assert publication['status'] == 'COMMITTED', publication
assert publication['client_revision'] == state['config_revision'], publication
print(f'USER_RULES_MUTATION_ROLLBACK=PASS config_revision={state["config_revision"]}')
PY
