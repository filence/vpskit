#!/usr/bin/env bash
set -euo pipefail

/usr/local/bin/vpskit rules show
/usr/local/bin/vpskit rules refresh --yes
/usr/local/bin/vpskit subscription status

python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text())
assert state['schema_version'] == 8, state
assert state['rules']['source_mode'] == 'managed', state
revision = state['rules']['revision']
manifest = json.loads(Path(f'/var/lib/vpskit/rules/r{revision:04d}/manifest.json').read_text())
assert manifest['revision'] == revision, manifest
assert len(manifest['sources']) == 20, manifest
assert all(item['sha256'] and item['bytes'] > 0 for item in manifest['sources']), manifest
config = Path('/etc/vpskit/exports/mihomo.yaml').read_text()
assert '/rules/anti-ad' in config, config
assert 'raw.githubusercontent.com/ACL4SSR' not in config, config
assert 'https://anti-ad.net/' not in config, config
print(f'MANAGED_RULE_CACHE=PASS revision={revision} sources={len(manifest["sources"])}')
PY
