#!/usr/bin/env bash
set -Eeuo pipefail

readonly archive='/root/v0.1.1-lab.1-linux-amd64.tar.gz'
readonly archive_sha256='f2cb20111650a1af96e29a5c127863faccb70fe9613b3b76ca8783c9b55d1034'
readonly previous_root='/root/vpskit-v011-upgrade'
readonly recovery_root='/root/vpskit-v011-migration-recovery'

test -f "$archive"
test -f "$previous_root/migrate-apply-first.json"
printf '%s  %s\n' "$archive_sha256" "$archive" | sha256sum -c --strict >/dev/null
rm -rf -- "$recovery_root"
install -d -m 0700 "$recovery_root"
tar -xzf "$archive" -C "$recovery_root"
bundle="$recovery_root/v0.1.1-lab.1"
chmod 0755 "$bundle/vpskit"
transaction_id="$(python3 - "$previous_root/migrate-apply-first.json" <<'PY'
import json
import sys
print(json.load(open(sys.argv[1], encoding='utf-8'))['detail']['transaction_id'])
PY
)"
"$bundle/vpskit" rollback "$transaction_id" --yes >/dev/null
python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
assert state['schema_version'] == 6, state
assert state['vpskit_version'] == 'v0.1.0', state
assert state['config_revision'] == 1, state
assert state['node']['display_name'] == 'JP', state
PY
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0'
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
rm -rf -- "$recovery_root"
printf 'V011_MIGRATION_PROBE_RECOVERY=PASS\n'
