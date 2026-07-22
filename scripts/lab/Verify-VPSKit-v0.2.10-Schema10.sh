#!/usr/bin/env bash
set -euo pipefail
umask 077

readonly state_path='/var/lib/vpskit/state.json'
readonly state_before_sha256='5282bd1df9e6fccd45263ee544e6f1415c2a9bdb55565a13de94591523c4657a'
readonly publication_before_sha256='a3a5d819131372f06d0cb12a667f41eebb04f9f43c4006b6ad9fb1f5ecc45df1'
readonly upgrade_root='/root/vpskit-v0.2.10-lab.1-upgrade'
readonly archive='/root/v0.2.10-lab.1-linux-amd64.tar.gz'
readonly backup_binary='/var/lib/vpskit/backups/v0.2.10-lab.1-cli-upgrade/vpskit-before-v0.2.10-lab.1'

test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.2.10-lab.1'
test ! -e "$upgrade_root"
test ! -e "$archive"
test -x "$backup_binary"
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service

inventory="$(/usr/local/bin/vpskit instance list)"
subscription="$(/usr/local/bin/vpskit subscription status)"
python3 - "$inventory" "$subscription" "$state_path" <<'PY'
import json
import sys

inventory = json.loads(sys.argv[1])
subscription = json.loads(sys.argv[2])
state = json.load(open(sys.argv[3], encoding='utf-8'))
assert inventory['status'] == 'PASS', inventory
assert subscription['status'] == 'PASS', subscription
assert state['schema_version'] == 10, state
assert state['config_revision'] == 15, state
instances = {item['id']: item for item in inventory['detail']['instances']}
assert instances['reality-main']['adapter'] == 'xray', instances
assert instances['reality-main']['protocol'] == 'vless-reality', instances
assert instances['reality-main']['listen'] == {'network': 'tcp', 'port': 443}, instances
assert instances['hy2-backup']['adapter'] == 'sing-box', instances
assert instances['hy2-backup']['protocol'] == 'hysteria2', instances
assert instances['hy2-backup']['listen'] == {'network': 'udp', 'port': 443}, instances
PY

state_after_sha256="$(sha256sum "$state_path" | awk '{print $1}')"
publication_after_sha256="$(sha256sum /var/lib/vpskit/subscription-state.json | awk '{print $1}')"
test "$state_after_sha256" != "$state_before_sha256"
test "$publication_after_sha256" != "$publication_before_sha256"
printf 'SCHEMA10_VERIFY=PASS schema=10 config_revision=15 subscription_readback=PASS state_sha256=%s publication_sha256=%s\n' \
  "$state_after_sha256" "$publication_after_sha256"
