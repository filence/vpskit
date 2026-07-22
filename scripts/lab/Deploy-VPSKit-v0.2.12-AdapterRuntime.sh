#!/usr/bin/env bash
set -euo pipefail
umask 077
v='v0.2.12-lab.1'; a="/root/${v}-linux-amd64.tar.gz"; h='57b9a1c592131a032bc3d241be805cad6df6dae7a887bac879cbcfac8c5ff018'; r="/root/vpskit-${v}-upgrade"; b="/var/lib/vpskit/backups/${v}-cli-upgrade/vpskit-before-${v}"
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.2.11-lab.1'; test -f "$a"; test "$(sha256sum "$a" | awk '{print $1}')" = "$h"; test ! -e "$r"; test ! -e "$b"
systemctl is-active --quiet vpskit-xray.service; systemctl is-active --quiet vpskit-sing-box.service
s="$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"; p="$(sha256sum /var/lib/vpskit/subscription-state.json | awk '{print $1}')"; ok=false; installed=false
cleanup(){ c=$?; if [ "$ok" != true ] && [ "$installed" = true ] && [ -f "$b" ]; then install -m 0755 "$b" /usr/local/bin/vpskit; fi; rm -rf -- "$r"; if [ "$ok" = true ]; then rm -f -- "$a"; fi; trap - EXIT; exit "$c"; }; trap cleanup EXIT
install -d -m 0700 "$r"; install -d -m 0700 "$(dirname "$b")"; tar -xzf "$a" -C "$r"; "$r/$v/vpskit" bundle verify --dir "$r/$v" >/dev/null; install -m 0700 /usr/local/bin/vpskit "$b"; install -m 0755 "$r/$v/vpskit" /usr/local/bin/vpskit; installed=true
test "$(/usr/local/bin/vpskit version)" = "vpskit $v"; i="$(/usr/local/bin/vpskit instance list)"
python3 - "$i" <<'PY'
import json,sys
x=json.loads(sys.argv[1]); assert x['status']=='PASS',x
a={v['target']:v for v in x['detail']['adapters']}; assert a['reality']['service']=='vpskit-xray.service' and a['hysteria2']['service']=='vpskit-sing-box.service',a
PY
systemctl is-active --quiet vpskit-xray.service; systemctl is-active --quiet vpskit-sing-box.service; test "$s" = "$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"; test "$p" = "$(sha256sum /var/lib/vpskit/subscription-state.json | awk '{print $1}')"; ok=true; printf 'ADAPTER_RUNTIME_DEPLOY=PASS version=%s\n' "$v"
