#!/usr/bin/env bash
set -euo pipefail
umask 077

readonly version='v0.2.11-lab.1'
readonly archive="/root/${version}-linux-amd64.tar.gz"
readonly archive_sha256='6defe99bc89884bd89da3f7d1ac5deab426bc00e58323bf1c1b532a5ae2d565a'
readonly root="/root/vpskit-${version}-upgrade"
readonly bundle="${root}/${version}"
readonly backup="/var/lib/vpskit/backups/${version}-cli-upgrade"
readonly backup_binary="${backup}/vpskit-before-${version}"
readonly state_path='/var/lib/vpskit/state.json'
readonly publication_path='/var/lib/vpskit/subscription-state.json'

test -f "$archive"
test "$(sha256sum "$archive" | awk '{print $1}')" = "$archive_sha256"
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.2.10-lab.1'
test ! -e "$root"
test ! -e "$backup_binary"
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service

state_before="$(sha256sum "$state_path" | awk '{print $1}')"
readonly state_before
publication_before="$(sha256sum "$publication_path" | awk '{print $1}')"
readonly publication_before
installed=false
completed=false
cleanup() {
  status=$?
  if [ "$completed" != true ] && [ "$installed" = true ] && [ -f "$backup_binary" ]; then
    install -m 0755 "$backup_binary" /usr/local/bin/vpskit
  fi
  rm -rf -- "$root"
  if [ "$completed" = true ]; then rm -f -- "$archive"; fi
  trap - EXIT
  exit "$status"
}
trap cleanup EXIT

install -d -m 0700 "$root"
install -d -m 0700 "$backup"
tar -xzf "$archive" -C "$root"
"$bundle/vpskit" bundle verify --dir "$bundle" >/dev/null
install -m 0700 /usr/local/bin/vpskit "$backup_binary"
install -m 0755 "$bundle/vpskit" /usr/local/bin/vpskit
installed=true
test "$(/usr/local/bin/vpskit version)" = "vpskit ${version}"

inventory="$(/usr/local/bin/vpskit instance list)"
python3 - "$inventory" "$state_path" <<'PY'
import json
import sys
inventory = json.loads(sys.argv[1])
state = json.load(open(sys.argv[2], encoding='utf-8'))
assert inventory['status'] == 'PASS', inventory
assert state['schema_version'] == 10 and state['config_revision'] == 15, state
entries = {item['id']: item for item in inventory['detail']['instances']}
assert entries['reality-main']['adapter'] == 'xray' and entries['reality-main']['listen']['network'] == 'tcp', entries
assert entries['hy2-backup']['adapter'] == 'sing-box' and entries['hy2-backup']['listen']['network'] == 'udp', entries
adapters = {item['target']: item for item in inventory['detail']['adapters']}
assert adapters['reality']['service'] == 'vpskit-xray.service', adapters
assert adapters['hysteria2']['service'] == 'vpskit-sing-box.service', adapters
PY

systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
test "$state_before" = "$(sha256sum "$state_path" | awk '{print $1}')"
test "$publication_before" = "$(sha256sum "$publication_path" | awk '{print $1}')"
completed=true
printf 'ADAPTER_REGISTRY_DEPLOY=PASS version=%s state_sha256=%s publication_sha256=%s\n' "$version" "$state_before" "$publication_before"
