#!/usr/bin/env bash
set -euo pipefail

readonly version='v0.2.7-lab.1'
readonly archive="/root/${version}-linux-amd64.tar.gz"
readonly archive_sha256='637384c4bb2de3e05031e7b0a83006caca8b6b68bfe92a95d75ade23204205eb'
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

state_before="$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
readonly state_before
publication_before="$(sha256sum /var/lib/vpskit/subscription-state.json | awk '{print $1}')"
readonly publication_before
install -d -m 0700 "$root"
install -d -m 0700 "$backup"
tar -xzf "$archive" -C "$root"
"$bundle/vpskit" bundle verify --dir "$bundle"
install -m 0700 /usr/local/bin/vpskit "$backup_binary"
install -m 0755 "$bundle/vpskit" /usr/local/bin/vpskit
test "$(/usr/local/bin/vpskit version)" = "vpskit ${version}"

/usr/local/bin/vpskit hysteria2 salamander plan >/root/vpskit-salamander-plan.json
python3 - <<'PY'
import json
from pathlib import Path

result = json.loads(Path('/root/vpskit-salamander-plan.json').read_text(encoding='utf-8'))
detail = result['detail']
assert result['status'] == 'PASS', result
assert detail['enabled'] is False, detail
assert detail['server_minimum'] == '1.13.0', detail
assert detail['manual_validation_required'] is True, detail
PY
test "$state_before" = "$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
test "$publication_before" = "$(sha256sum /var/lib/vpskit/subscription-state.json | awk '{print $1}')"
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
printf 'SALAMANDER_PREVIEW_READ_ONLY=PASS\n'
