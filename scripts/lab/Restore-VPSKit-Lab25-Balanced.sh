#!/usr/bin/env bash
set -Eeuo pipefail

profile="$(python3 -c 'import json; print(json.load(open("/var/lib/vpskit/state.json"))["profile"])')"
case "$profile" in
    balanced) ;;
    reality-only) /usr/local/bin/vpskit instance enable hysteria2 ;;
    hysteria2-only) /usr/local/bin/vpskit instance enable reality ;;
    *) printf 'unsupported recovery profile=%s\n' "$profile" >&2; exit 1 ;;
esac

python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
assert state['profile'] == 'balanced', state['profile']
assert state['reality']['enabled'] is True
assert state['hysteria2']['enabled'] is True
assert state['reality']['listen_port'] == 443
assert state['hysteria2']['listen_port'] == 443
print('LAB25_BALANCED_RESTORED=PASS')
PY
/usr/local/bin/vpskit doctor
