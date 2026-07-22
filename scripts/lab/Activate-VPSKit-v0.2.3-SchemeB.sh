#!/usr/bin/env bash
set -euo pipefail

test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.2.3-lab.1'
/usr/local/bin/vpskit rules plan --profile acl4ssr
/usr/local/bin/vpskit rules apply --profile acl4ssr --yes
/usr/local/bin/vpskit rules refresh --yes
/usr/local/bin/vpskit subscription status
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service

python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text())
publication = json.loads(Path('/var/lib/vpskit/subscription-state.json').read_text())
manifest = json.loads(Path(f'/var/lib/vpskit/rules/r{state["rules"]["revision"]:04d}/manifest.json').read_text())
config = Path('/etc/vpskit/exports/mihomo.yaml').read_text()
assert state['schema_version'] == 9, state
assert state['rules']['profile'] == 'acl4ssr', state
assert state['rules']['source_mode'] == 'managed', state
assert len(manifest['sources']) == 19, manifest
assert 'anti-AD:' not in config, config
assert 'RULE-SET,anti-AD,REJECT' not in config, config
assert '/rules/acl4ssr-openai' in config, config
assert publication['status'] == 'COMMITTED', publication
assert publication['client_revision'] == state['config_revision'], publication
print(f'SCHEME_B_MANAGED=PASS config_revision={state["config_revision"]} ruleset_revision={state["rules"]["revision"]}')
PY
