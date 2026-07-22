#!/usr/bin/env bash
set -euo pipefail
umask 077

v='v0.2.13-lab.1'
a="/root/${v}-linux-amd64.tar.gz"
h='d7466a377d138459010720c75137d76365a661a009b060207ca41f957f1c0248'
r="/root/vpskit-${v}-upgrade"
b="/var/lib/vpskit/backups/${v}-cli-upgrade/vpskit-before-${v}"

test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.2.12-lab.1'
test -f "$a"
test "$(sha256sum "$a" | awk '{print $1}')" = "$h"
test ! -e "$r"
test ! -e "$b"
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service

s="$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
p="$(sha256sum /var/lib/vpskit/subscription-state.json | awk '{print $1}')"
ok=false
installed=false
cleanup() {
  c=$?
  if [ "$ok" != true ] && [ "$installed" = true ] && [ -f "$b" ]; then
    install -m 0755 "$b" /usr/local/bin/vpskit
  fi
  rm -rf -- "$r"
  if [ "$ok" = true ]; then rm -f -- "$a"; fi
  trap - EXIT
  exit "$c"
}
trap cleanup EXIT

install -d -m 0700 "$r"
install -d -m 0700 "$(dirname "$b")"
tar -xzf "$a" -C "$r"
"$r/$v/vpskit" bundle verify --dir "$r/$v" >/dev/null
install -m 0700 /usr/local/bin/vpskit "$b"
install -m 0755 "$r/$v/vpskit" /usr/local/bin/vpskit
installed=true
test "$(/usr/local/bin/vpskit version)" = "vpskit $v"

/usr/local/bin/vpskit hysteria2 recommend --server-mbps 500 --client-mbps 300 >/root/vpskit-hysteria2-recommend.json
python3 - <<'PY'
import json
from pathlib import Path
payload = json.loads(Path('/root/vpskit-hysteria2-recommend.json').read_text(encoding='utf-8'))
assert payload['status'] == 'PASS', payload
detail = payload['detail']
assert detail['read_only'] is True and detail['service_restart'] is False, detail
assert detail['test_plan']['bottleneck_mbps'] == 300, detail
assert detail['test_plan']['conservative_test_ceiling_mbps'] == 255, detail
assert detail['server_config']['up_down_mbps'] == 'KEEP_UNSET', detail
assert detail['feature_gate']['bbr_profile']['status'] == 'BLOCKED', detail
PY

systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
test "$s" = "$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
test "$p" = "$(sha256sum /var/lib/vpskit/subscription-state.json | awk '{print $1}')"
ok=true
printf 'HYSTERIA2_RECOMMEND_DEPLOY=PASS version=%s\n' "$v"
