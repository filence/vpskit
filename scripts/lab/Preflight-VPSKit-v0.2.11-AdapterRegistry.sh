#!/usr/bin/env bash
set -euo pipefail
umask 077

test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.2.10-lab.1'
test -f /var/lib/vpskit/state.json
test -f /var/lib/vpskit/subscription-state.json
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service

inventory="$(/usr/local/bin/vpskit instance list)"
python3 - "$inventory" /var/lib/vpskit/state.json <<'PY'
import json
import sys
inventory = json.loads(sys.argv[1])
state = json.load(open(sys.argv[2], encoding='utf-8'))
assert inventory['status'] == 'PASS', inventory
assert state['schema_version'] == 10, state
assert state['config_revision'] == 15, state
PY

printf 'ADAPTER_REGISTRY_PREFLIGHT=PASS state_sha256=%s publication_sha256=%s\n' \
  "$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')" \
  "$(sha256sum /var/lib/vpskit/subscription-state.json | awk '{print $1}')"
