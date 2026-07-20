#!/usr/bin/env bash
set -euo pipefail
backup='/var/lib/vpskit/backups/BK-20260718-023744-256ca5'
tmp='/var/lib/vpskit/transactions/debug-restore-staging'
rm -rf -- "$tmp"
install -d -m 0700 "$tmp/generated"
cp -- "$backup/generated/sing-box.json" "$tmp/generated/sing-box.json"
sha256sum "$backup/generated/sing-box.json" "$tmp/generated/sing-box.json" /etc/vpskit/generated/sing-box.json
python3 - <<'PY'
import json
import pathlib
state=json.loads(pathlib.Path('/var/lib/vpskit/backups/BK-20260718-023744-256ca5/state.json').read_text())
print(state['config_sha256'])
PY
rm -rf -- "$tmp"
echo 'DEBUG_RESTORE_STAGING=PASS'
