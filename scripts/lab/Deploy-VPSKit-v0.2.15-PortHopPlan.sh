#!/usr/bin/env bash
set -euo pipefail
umask 077
v='v0.2.15-lab.1'; a="/root/${v}-linux-amd64.tar.gz"; h='e84e8f3b90f38e016a0a38a9d3847db4ac784f5f2854c83ac558036fcf78091d'; r="/root/vpskit-${v}-upgrade"; b="/var/lib/vpskit/backups/${v}-cli-upgrade/vpskit-before-${v}"
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.2.14-lab.1'; test -f "$a" && test "$(sha256sum "$a" | awk '{print $1}')" = "$h"; test ! -e "$r" && test ! -e "$b"
systemctl is-active --quiet vpskit-xray.service; systemctl is-active --quiet vpskit-sing-box.service
s="$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"; p="$(sha256sum /var/lib/vpskit/subscription-state.json | awk '{print $1}')"; ok=false; installed=false
cleanup(){ c=$?; if [ "$ok" != true ] && [ "$installed" = true ] && [ -f "$b" ]; then install -m 0755 "$b" /usr/local/bin/vpskit; fi; rm -rf -- "$r"; if [ "$ok" = true ]; then rm -f -- "$a"; fi; trap - EXIT; exit "$c"; }; trap cleanup EXIT
mkdir -p -m 0700 "$r" "$(dirname "$b")"; tar -xzf "$a" -C "$r"; "$r/$v/vpskit" bundle verify --dir "$r/$v" >/dev/null; install -m 0700 /usr/local/bin/vpskit "$b"; install -m 0755 "$r/$v/vpskit" /usr/local/bin/vpskit; installed=true
test "$(/usr/local/bin/vpskit version)" = "vpskit $v"
/usr/local/bin/vpskit hysteria2 port-hop plan --range 20000-20010 --hop-interval 30 >/root/vpskit-port-hop-plan.json
python3 - <<'PY'
import json
from pathlib import Path
x=json.loads(Path('/root/vpskit-port-hop-plan.json').read_text())
assert x['status']=='PASS',x
d=x['detail']; assert d['read_only'] is True and d['apply_available'] is False and d['status']=='BLOCKED',d
assert d['range']=='20000-20010' and d['port_count']==11 and d['hop_interval_seconds']==30,d
assert d['backend']['listener_port']==443 and d['implementation_gate']['managed_redirect']=='NOT_IMPLEMENTED',d
PY
systemctl is-active --quiet vpskit-xray.service; systemctl is-active --quiet vpskit-sing-box.service; test "$s" = "$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"; test "$p" = "$(sha256sum /var/lib/vpskit/subscription-state.json | awk '{print $1}')"
ok=true; printf 'PORTHOP_PLAN_DEPLOY=PASS version=%s\n' "$v"
