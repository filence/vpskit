#!/usr/bin/env bash
set -euo pipefail
umask 077
v='v0.2.17-lab.1'; a="/root/${v}-linux-amd64.tar.gz"; h='bf87e4de9ee2e2d76e6c01ca8e6e1c47256b062c157f969925062c90f6acbb56'; r="/root/vpskit-${v}-upgrade"; b="/var/lib/vpskit/backups/${v}-cli-upgrade/vpskit-before-${v}"
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.2.16-lab.1'; test -f "$a" && test "$(sha256sum "$a"|awk '{print $1}')" = "$h"; test ! -e "$r" && test ! -e "$b"; command -v nft >/dev/null; systemctl is-active --quiet vpskit-xray.service;systemctl is-active --quiet vpskit-sing-box.service
s="$(sha256sum /var/lib/vpskit/state.json|awk '{print $1}')";p="$(sha256sum /var/lib/vpskit/subscription-state.json|awk '{print $1}')";ok=false;installed=false
cleanup(){ c=$?;if [ "$ok" != true ]&&[ "$installed" = true ]&&[ -f "$b" ];then install -m 0755 "$b" /usr/local/bin/vpskit;fi;rm -rf -- "$r";if [ "$ok" = true ];then rm -f -- "$a";fi;trap - EXIT;exit "$c";};trap cleanup EXIT
mkdir -p -m 0700 "$r" "$(dirname "$b")";tar -xzf "$a" -C "$r";"$r/$v/vpskit" bundle verify --dir "$r/$v" >/dev/null;install -m 0700 /usr/local/bin/vpskit "$b";install -m 0755 "$r/$v/vpskit" /usr/local/bin/vpskit;installed=true;test "$(/usr/local/bin/vpskit version)" = "vpskit $v"
/usr/local/bin/vpskit hysteria2 port-hop prepare --range 20000-20010 --backend-port 443 --yes >/root/vpskit-port-hop-prepare.json
/usr/local/bin/vpskit hysteria2 port-hop status >/root/vpskit-port-hop-status.json
python3 - <<'PY'
import json
from pathlib import Path
p=json.loads(Path('/root/vpskit-port-hop-prepare.json').read_text());s=json.loads(Path('/root/vpskit-port-hop-status.json').read_text())
assert p['status']=='PASS' and p['detail']['enabled'] is False,p
assert s['status']=='PASS' and s['detail']['nft_config_present'] and s['detail']['unit_present'] and s['detail']['unit_state']!='active',s
PY
systemctl is-active --quiet vpskit-xray.service;systemctl is-active --quiet vpskit-sing-box.service;test "$s" = "$(sha256sum /var/lib/vpskit/state.json|awk '{print $1}')";test "$p" = "$(sha256sum /var/lib/vpskit/subscription-state.json|awk '{print $1}')"; ok=true;printf 'PORTHOP_PREPARE_DEPLOY=PASS version=%s\n' "$v"
