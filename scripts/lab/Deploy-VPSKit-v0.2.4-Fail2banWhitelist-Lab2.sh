#!/usr/bin/env bash
set -euo pipefail

readonly version='v0.2.4-lab.2'
readonly archive="/root/${version}-linux-amd64.tar.gz"
readonly archive_sha256='f93d5d726150efa21f340813874408fbef753518dd51ec0ded73dd346d92ae79'
readonly root="/root/vpskit-${version}-upgrade"
readonly bundle="${root}/${version}"
readonly backup="/var/lib/vpskit/backups/${version}-cli-upgrade"
readonly backup_binary="${backup}/vpskit-before-${version}"
readonly test_cidr='198.51.100.24'

cleanup() {
  /usr/local/bin/vpskit security fail2ban whitelist remove --cidr "$test_cidr" --yes >/dev/null 2>&1 || true
}
trap cleanup EXIT

test -f "$archive"
test -x /usr/local/bin/vpskit
test "$(sha256sum "$archive" | awk '{print $1}')" = "$archive_sha256"
if [ -e "$root" ] || [ -e "$backup_binary" ]; then
  printf 'UPGRADE_REFUSED existing_lab_target\n' >&2
  exit 1
fi

systemctl is-active --quiet fail2ban.service
install -d -m 0700 "$root"
install -d -m 0700 "$backup"
tar -xzf "$archive" -C "$root"
"$bundle/vpskit" bundle verify --dir "$bundle"
install -m 0700 /usr/local/bin/vpskit "$backup_binary"
install -m 0755 "$bundle/vpskit" /usr/local/bin/vpskit

test "$(/usr/local/bin/vpskit version)" = "vpskit ${version}"
/usr/local/bin/vpskit security fail2ban whitelist list
/usr/local/bin/vpskit security fail2ban whitelist add --cidr "$test_cidr" --yes
fail2ban-client -d >/dev/null
fail2ban-client status sshd >/dev/null
/usr/local/bin/vpskit security fail2ban whitelist remove --cidr "$test_cidr" --yes
/usr/local/bin/vpskit security fail2ban whitelist list
/usr/local/bin/vpskit security fail2ban status
systemctl is-active --quiet fail2ban.service
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service

python3 - <<'PY'
import json
from pathlib import Path

owner = json.loads(Path('/var/lib/vpskit/fail2ban.json').read_text())
contents = Path('/etc/fail2ban/jail.d/vpskit-sshd.conf').read_text()
assert owner['schema_version'] == 2, owner
assert owner.get('whitelist', []) == [], owner
assert '198.51.100.24/32' not in contents, contents
print('FAIL2BAN_WHITELIST_LIFECYCLE=PASS')
PY
