#!/usr/bin/env bash
set -euo pipefail

readonly version='v0.2.5-lab.1'
readonly archive="/root/${version}-linux-amd64.tar.gz"
readonly archive_sha256='4788e7cb18f28db6cc8233beca857f04d5c69d4a6664306f792b93e58605c0d9'
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

install -d -m 0700 "$root"
install -d -m 0700 "$backup"
tar -xzf "$archive" -C "$root"
"$bundle/vpskit" bundle verify --dir "$bundle"
install -m 0700 /usr/local/bin/vpskit "$backup_binary"
install -m 0755 "$bundle/vpskit" /usr/local/bin/vpskit
test "$(/usr/local/bin/vpskit version)" = "vpskit ${version}"

read -r active_revision candidate_revision < <(python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text())
assert state['rules']['profile'] == 'acl4ssr', state
assert state['rules']['source_mode'] == 'managed', state
print(state['rules']['revision'], state['rules']['revision'] + 1)
PY
)
active_root="/var/lib/vpskit/rules/r$(printf '%04d' "$active_revision")"
readonly active_root
candidate_root="/var/lib/vpskit/rules/r$(printf '%04d' "$candidate_revision")"
readonly candidate_root
test -f "$active_root/manifest.json"
test ! -e "$candidate_root"

state_before="$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
readonly state_before
publication_before="$(sha256sum /var/lib/vpskit/subscription-state.json | awk '{print $1}')"
readonly publication_before
manifest_before="$(sha256sum "$active_root/manifest.json" | awk '{print $1}')"
readonly manifest_before
failure_log="$(mktemp)"
trap 'rm -f "$failure_log"' EXIT

if env -u NO_PROXY -u no_proxy \
  HTTPS_PROXY='http://127.0.0.1:1' HTTP_PROXY='http://127.0.0.1:1' ALL_PROXY='http://127.0.0.1:1' \
  /usr/local/bin/vpskit rules refresh --yes >"$failure_log" 2>&1; then
  printf 'EXPECTED_RULE_REFRESH_FAILURE_DID_NOT_OCCUR\n' >&2
  exit 1
fi

test "$state_before" = "$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
test "$publication_before" = "$(sha256sum /var/lib/vpskit/subscription-state.json | awk '{print $1}')"
test "$manifest_before" = "$(sha256sum "$active_root/manifest.json" | awk '{print $1}')"
test ! -e "$candidate_root"
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service

python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text())
publication = json.loads(Path('/var/lib/vpskit/subscription-state.json').read_text())
assert state['rules']['source_mode'] == 'managed', state
assert publication['status'] == 'COMMITTED', publication
print(f'RULE_REFRESH_FAILURE_PRESERVES_ACTIVE=PASS revision={state["rules"]["revision"]} config_revision={state["config_revision"]}')
PY
