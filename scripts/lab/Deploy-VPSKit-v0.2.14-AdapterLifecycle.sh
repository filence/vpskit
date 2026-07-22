#!/usr/bin/env bash
set -euo pipefail
umask 077

v='v0.2.14-lab.1'
a="/root/${v}-linux-amd64.tar.gz"
h='0dc4338942a6116b501162f214a022cfa040f108b35e1d3139bf634f20b851f2'
r="/root/vpskit-${v}-upgrade"
b="/var/lib/vpskit/backups/${v}-cli-upgrade/vpskit-before-${v}"

test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.2.13-lab.1'
test -f "$a" && test "$(sha256sum "$a" | awk '{print $1}')" = "$h"
test ! -e "$r" && test ! -e "$b"
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
s="$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
p="$(sha256sum /var/lib/vpskit/subscription-state.json | awk '{print $1}')"
ok=false; installed=false
cleanup() {
  c=$?
  if [ "$ok" != true ] && [ "$installed" = true ] && [ -f "$b" ]; then install -m 0755 "$b" /usr/local/bin/vpskit; fi
  rm -rf -- "$r"
  if [ "$ok" = true ]; then rm -f -- "$a"; fi
  trap - EXIT
  exit "$c"
}
trap cleanup EXIT
mkdir -p -m 0700 "$r" "$(dirname "$b")"
tar -xzf "$a" -C "$r"
"$r/$v/vpskit" bundle verify --dir "$r/$v" >/dev/null
install -m 0700 /usr/local/bin/vpskit "$b"
install -m 0755 "$r/$v/vpskit" /usr/local/bin/vpskit
installed=true
test "$(/usr/local/bin/vpskit version)" = "vpskit $v"
/usr/local/bin/vpskit instance list >/root/vpskit-instance-list-v0.2.14.json
/usr/local/bin/vpskit doctor >/root/vpskit-doctor-v0.2.14.json
python3 - <<'PY'
import json
from pathlib import Path
instances=json.loads(Path('/root/vpskit-instance-list-v0.2.14.json').read_text())
doctor=json.loads(Path('/root/vpskit-doctor-v0.2.14.json').read_text())
assert instances['status']=='PASS' and doctor['status']=='PASS', (instances,doctor)
assert len(instances['detail']['instances']) == 2, instances
PY
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
test "$s" = "$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
test "$p" = "$(sha256sum /var/lib/vpskit/subscription-state.json | awk '{print $1}')"
ok=true
printf 'ADAPTER_LIFECYCLE_DEPLOY=PASS version=%s\n' "$v"
