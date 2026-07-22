#!/usr/bin/env bash
set -euo pipefail
umask 077

version='v0.2.21-lab.1'
archive="/root/${version}-linux-amd64.tar.gz"
archive_sha256='95647be2b1e1bc3f518c70771eaf580afe980db9b4d158b64eecc09eafcd8f59'
staging="/root/vpskit-${version}-upgrade"
rollback_binary="/var/lib/vpskit/backups/${version}-cli-upgrade/vpskit-before-${version}"
ok=false
installed=false
state_mutated=false

cleanup() {
  exit_code=$?
  if [ "$ok" != true ] && [ "$state_mutated" != true ] && [ "$installed" = true ] && [ -f "$rollback_binary" ]; then
    install -m 0755 "$rollback_binary" /usr/local/bin/vpskit
  fi
  rm -rf -- "$staging"
  if [ "$ok" = true ]; then
    rm -f -- "$archive"
  fi
  trap - EXIT
  exit "$exit_code"
}
trap cleanup EXIT

test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.2.20-lab.1'
test "$(sha256sum "$archive" | awk '{print $1}')" = "$archive_sha256"
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-hysteria2-port-hop.service

install -d -m 0700 "$staging"
install -d -m 0700 "$(dirname "$rollback_binary")"
tar -xzf "$archive" -C "$staging"
"$staging/$version/vpskit" bundle verify --dir "$staging/$version" >/dev/null
install -m 0700 /usr/local/bin/vpskit "$rollback_binary"
install -m 0755 "$staging/$version/vpskit" /usr/local/bin/vpskit
installed=true
test "$(/usr/local/bin/vpskit version)" = "vpskit $version"

/usr/local/bin/vpskit hysteria2 port-hop enable --range 20000-20010 --hop-interval 30 --yes >/root/vpskit-port-hop-enable.json
state_mutated=true
/usr/local/bin/vpskit subscription status >/root/vpskit-port-hop-subscription-status.json

python3 - <<'PY'
import json
from pathlib import Path

enable = json.loads(Path('/root/vpskit-port-hop-enable.json').read_text())
assert enable['status'] == 'PASS', enable
assert enable['detail']['result'] == 'ENABLED', enable
assert enable['detail']['range'] == '20000-20010', enable
assert enable['detail']['hop_interval_seconds'] == 30, enable
assert enable['detail']['client_update_required'] is True, enable
assert enable['detail']['service_restart'] is False, enable
assert enable['detail']['subscription_publish']['status'] == 'PASS', enable

state = json.loads(Path('/var/lib/vpskit/state.json').read_text())
hop = state['hysteria2']['port_hopping']
assert hop == {'enabled': True, 'range_start': 20000, 'range_end': 20010, 'hop_interval_seconds': 30}, hop

subscription = json.loads(Path('/root/vpskit-port-hop-subscription-status.json').read_text())
assert subscription['status'] == 'PASS', subscription
assert subscription['detail']['readback'] == 'PASS', subscription

mihomo = Path('/etc/vpskit/exports/mihomo.yaml').read_text()
assert 'ports: 20000-20010' in mihomo, mihomo
assert 'hop-interval: 30' in mihomo, mihomo

singbox = json.loads(Path('/etc/vpskit/exports/sing-box-hysteria2.json').read_text())
outbound = singbox['outbounds'][0]
assert outbound['server_ports'] == ['20000-20010'], outbound
assert outbound['hop_interval'] == '30s', outbound
assert 'server_port' not in outbound, outbound

share_links = Path('/etc/vpskit/exports/share-links.txt').read_text()
assert 'ports=20000-20010' in share_links and 'hop-interval=30' in share_links, share_links
PY

systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-hysteria2-port-hop.service
nft list table inet vpskit_hysteria2_port_hop >/root/vpskit-port-hop-nft.txt

ok=true
printf 'PORTHOP_CLIENT_EXPORT_DEPLOY=PASS version=%s\n' "$version"
