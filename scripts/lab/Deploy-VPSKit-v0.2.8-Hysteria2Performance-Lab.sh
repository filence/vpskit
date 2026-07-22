#!/usr/bin/env bash
set -euo pipefail

readonly version='v0.2.8-lab.1'
readonly archive="/root/${version}-linux-amd64.tar.gz"
readonly archive_sha256='bd19e4bc337da470bfba063860e8f7217de8c12d68351f04f9d559a10e5ca96a'
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

readonly state_before="$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
readonly publication_before="$(sha256sum /var/lib/vpskit/subscription-state.json | awk '{print $1}')"
mkdir -p -m 0700 "$root" "$backup"
tar -xzf "$archive" -C "$root"
"$bundle/vpskit" bundle verify --dir "$bundle"
install -m 0700 /usr/local/bin/vpskit "$backup_binary"
install -m 0755 "$bundle/vpskit" /usr/local/bin/vpskit
test "$(/usr/local/bin/vpskit version)" = "vpskit ${version}"

/usr/local/bin/vpskit hysteria2 performance inspect >/root/vpskit-hysteria2-performance.json
python3 - <<'PY'
import json
from pathlib import Path

result = json.loads(Path('/root/vpskit-hysteria2-performance.json').read_text(encoding='utf-8'))
detail = result['detail']
assert result['status'] == 'PASS', result
assert detail['runtime']['listener_present'] is True, detail
assert detail['process']['status'] == 'AVAILABLE', detail
assert detail['kernel_congestion']['status'] == 'AVAILABLE', detail
assert detail['client_path_measurement']['status'] == 'NOT_MEASURED', detail
PY
test "$state_before" = "$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
test "$publication_before" = "$(sha256sum /var/lib/vpskit/subscription-state.json | awk '{print $1}')"
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
printf 'HYSTERIA2_PERFORMANCE_READ_ONLY=PASS\n'
