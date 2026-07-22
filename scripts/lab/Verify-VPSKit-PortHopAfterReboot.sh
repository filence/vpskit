#!/usr/bin/env bash
set -euo pipefail

systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-hysteria2-port-hop.service
nft list table inet vpskit_hysteria2_port_hop >/root/vpskit-port-hop-nft-after-reboot.txt
/usr/local/bin/vpskit hysteria2 port-hop status >/root/vpskit-port-hop-status-after-reboot.json
/usr/local/bin/vpskit subscription status >/root/vpskit-port-hop-subscription-after-reboot.json
python3 - <<'PY'
import json
from pathlib import Path

port_hop = json.loads(Path('/root/vpskit-port-hop-status-after-reboot.json').read_text())
assert port_hop['status'] == 'PASS', port_hop
assert port_hop['detail']['unit_state'] == 'active', port_hop

subscription = json.loads(Path('/root/vpskit-port-hop-subscription-after-reboot.json').read_text())
assert subscription['status'] == 'PASS', subscription
assert subscription['detail']['readback'] == 'PASS', subscription
PY
printf '%s\n' 'VPSKIT_PORTHOP_REBOOT_VERIFY=PASS'
