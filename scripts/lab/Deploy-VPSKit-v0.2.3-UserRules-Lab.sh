#!/usr/bin/env bash
set -euo pipefail

readonly version='v0.2.3-lab.1'
readonly archive="/root/${version}-linux-amd64.tar.gz"
readonly archive_sha256='fb8d8dd529a25f18f3be94feddf9b2aceea3efd1deaccc7a7e86fa66b22395c3'
readonly root="/root/vpskit-${version}-upgrade"
readonly bundle="${root}/${version}"
readonly backup="/var/lib/vpskit/backups/${version}-cli-upgrade"
readonly backup_binary="${backup}/vpskit-before-${version}"

test -f "$archive"
test -x /usr/local/bin/vpskit
test "$(sha256sum "$archive" | awk '{print $1}')" = "$archive_sha256"
if [ -e "$root" ] || [ -e "$backup_binary" ]; then
  printf 'UPGRADE_REFUSED existing_lab_target\n' >&2
  exit 1
fi

systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
install -d -m 0700 "$root"
install -d -m 0700 "$backup"
tar -xzf "$archive" -C "$root"
"$bundle/vpskit" bundle verify --dir "$bundle"
install -m 0700 /usr/local/bin/vpskit "$backup_binary"
install -m 0755 "$bundle/vpskit" /usr/local/bin/vpskit

test "$(/usr/local/bin/vpskit version)" = "vpskit ${version}"
/usr/local/bin/vpskit migrate plan
/usr/local/bin/vpskit migrate apply --yes
/usr/local/bin/vpskit rules whitelist list
/usr/local/bin/vpskit rules custom list
/usr/local/bin/vpskit rules custom check
/usr/local/bin/vpskit subscription status
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service

python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text())
assert state['schema_version'] == 9, state
assert state['rules']['source_mode'] == 'managed', state
assert state['rules'].get('user_rules', []) == [], state
print(f'USER_RULES_SCHEMA_MIGRATION=PASS config_revision={state["config_revision"]}')
PY

printf 'UPGRADE_V023_USER_RULES=PASS backup=%s\n' "$backup_binary"
