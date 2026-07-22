#!/usr/bin/env bash
set -euo pipefail

assert_port_hop_state() {
  expected_enabled="$1"
  python3 - "$expected_enabled" <<'PY'
import json
import sys
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text())
hop = state['hysteria2']['port_hopping']
expected = sys.argv[1] == 'true'
assert hop.get('enabled', False) is expected, hop
if expected:
    assert hop == {'enabled': True, 'range_start': 20000, 'range_end': 20010, 'hop_interval_seconds': 30}, hop
PY
}

systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-hysteria2-port-hop.service
assert_port_hop_state true

/usr/local/bin/vpskit hysteria2 port-hop disable --yes >/root/vpskit-port-hop-disable.json
assert_port_hop_state false
systemctl is-active --quiet vpskit-hysteria2-port-hop.service
/usr/local/bin/vpskit hysteria2 port-hop deactivate --yes >/root/vpskit-port-hop-deactivate.json
if systemctl is-active --quiet vpskit-hysteria2-port-hop.service; then
  echo 'port-hop unit unexpectedly remains active' >&2
  exit 1
fi
if nft list table inet vpskit_hysteria2_port_hop >/dev/null 2>&1; then
  echo 'port-hop nft table unexpectedly remains present' >&2
  exit 1
fi

/usr/local/bin/vpskit hysteria2 port-hop prepare --range 20000-20010 --backend-port 443 --yes >/root/vpskit-port-hop-reprepare.json
/usr/local/bin/vpskit hysteria2 port-hop activate --yes >/root/vpskit-port-hop-reactivate.json
/usr/local/bin/vpskit hysteria2 port-hop enable --range 20000-20010 --hop-interval 30 --yes >/root/vpskit-port-hop-reenable.json
assert_port_hop_state true
systemctl is-active --quiet vpskit-hysteria2-port-hop.service
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
nft list table inet vpskit_hysteria2_port_hop >/root/vpskit-port-hop-nft-lifecycle.txt
/usr/local/bin/vpskit subscription status >/root/vpskit-port-hop-subscription-lifecycle.json

python3 - <<'PY'
import json
from pathlib import Path

for name in ('disable', 'deactivate', 'reprepare', 'reactivate', 'reenable'):
    value = json.loads(Path(f'/root/vpskit-port-hop-{name}.json').read_text())
    assert value['status'] == 'PASS', value

reenable = json.loads(Path('/root/vpskit-port-hop-reenable.json').read_text())
assert reenable['detail']['subscription_publish']['status'] == 'PASS', reenable
assert reenable['detail']['range'] == '20000-20010', reenable

subscription = json.loads(Path('/root/vpskit-port-hop-subscription-lifecycle.json').read_text())
assert subscription['status'] == 'PASS', subscription
assert subscription['detail']['readback'] == 'PASS', subscription

mihomo = Path('/etc/vpskit/exports/mihomo.yaml').read_text()
assert 'ports: 20000-20010' in mihomo and 'hop-interval: 30' in mihomo, mihomo
PY

printf '%s\n' 'VPSKIT_PORTHOP_LIFECYCLE=PASS'
