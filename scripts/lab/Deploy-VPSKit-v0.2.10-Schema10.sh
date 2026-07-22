#!/usr/bin/env bash
set -euo pipefail
umask 077

readonly version='v0.2.10-lab.1'
readonly archive="/root/${version}-linux-amd64.tar.gz"
readonly archive_sha256='b746beecfb8f426a68b5088682b22adf7bce1012d0362d56e7fa5a27bdafe241'
readonly root="/root/vpskit-${version}-upgrade"
readonly bundle="${root}/${version}"
readonly backup="/var/lib/vpskit/backups/${version}-cli-upgrade"
readonly backup_binary="${backup}/vpskit-before-${version}"
readonly state_path='/var/lib/vpskit/state.json'
readonly publication_path='/var/lib/vpskit/subscription-state.json'

test -f "$archive"
test -x /usr/local/bin/vpskit
test "$(sha256sum "$archive" | awk '{print $1}')" = "$archive_sha256"
test ! -e "$root"
test ! -e "$backup_binary"
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.2.9-lab.1'
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service

readonly state_before_sha256="$(sha256sum "$state_path" | awk '{print $1}')"
readonly publication_before_sha256="$(sha256sum "$publication_path" | awk '{print $1}')"
readonly config_revision_before="$(python3 - "$state_path" <<'PY'
import json
import sys
print(json.load(open(sys.argv[1], encoding='utf-8'))['config_revision'])
PY
)"

installed_new_binary=false
completed=false
cleanup() {
  status=$?
  if [ "$completed" != true ] && [ "$installed_new_binary" = true ] && [ -f "$backup_binary" ]; then
    install -m 0755 "$backup_binary" /usr/local/bin/vpskit
  fi
  rm -rf -- "$root"
  if [ "$completed" = true ]; then
    rm -f -- "$archive"
  fi
  trap - EXIT
  exit "$status"
}
trap cleanup EXIT

mkdir -p -m 0700 "$root" "$backup"
tar -xzf "$archive" -C "$root"
"$bundle/vpskit" bundle verify --dir "$bundle" >/dev/null
install -m 0700 /usr/local/bin/vpskit "$backup_binary"
install -m 0755 "$bundle/vpskit" /usr/local/bin/vpskit
installed_new_binary=true
test "$(/usr/local/bin/vpskit version)" = "vpskit ${version}"

plan="$(/usr/local/bin/vpskit migrate plan)"
python3 - "$plan" <<'PY'
import json
import sys

result = json.loads(sys.argv[1])
assert result['status'] == 'PASS', result
detail = result['detail']
assert detail['source_schema'] == 9, detail
assert detail['target_schema'] == 10, detail
assert detail['migration_required'] is True, detail
assert any('instance inventory and adapter ownership' in step for step in detail['steps']), detail
PY

apply="$(/usr/local/bin/vpskit migrate apply --yes)"
python3 - "$apply" "$config_revision_before" <<'PY'
import json
import sys

result = json.loads(sys.argv[1])
expected_revision = int(sys.argv[2])
assert result['status'] == 'PASS', result
detail = result['detail']
assert detail['result'] == 'MIGRATED', detail
assert detail['source_schema'] == 9 and detail['state_schema'] == 10, detail
assert detail['config_revision'] == expected_revision, detail
assert detail['client_update_required'] is False, detail
PY

inventory="$(/usr/local/bin/vpskit instance list)"
python3 - "$inventory" "$state_path" "$config_revision_before" <<'PY'
import json
import sys

result = json.loads(sys.argv[1])
state = json.load(open(sys.argv[2], encoding='utf-8'))
expected_revision = int(sys.argv[3])
assert result['status'] == 'PASS', result
detail = result['detail']
assert detail['state_schema'] == 10, detail
instances = {item['id']: item for item in detail['instances']}
assert instances['reality-main']['adapter'] == 'xray', instances
assert instances['reality-main']['protocol'] == 'vless-reality', instances
assert instances['reality-main']['listen']['network'] == 'tcp', instances
assert instances['hy2-backup']['adapter'] == 'sing-box', instances
assert instances['hy2-backup']['protocol'] == 'hysteria2', instances
assert instances['hy2-backup']['listen']['network'] == 'udp', instances
assert state['schema_version'] == 10, state
assert state['config_revision'] == expected_revision, state
PY

systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
completed=true
printf 'SCHEMA10_DEPLOY=PASS version=%s state_before_sha256=%s state_after_sha256=%s publication_before_sha256=%s publication_after_sha256=%s config_revision=%s\n' \
  "$version" \
  "$state_before_sha256" \
  "$(sha256sum "$state_path" | awk '{print $1}')" \
  "$publication_before_sha256" \
  "$(sha256sum "$publication_path" | awk '{print $1}')" \
  "$config_revision_before"
