#!/usr/bin/env bash
set -euo pipefail

systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-hysteria2-port-hop.service
nft list table inet vpskit_hysteria2_port_hop >/dev/null
systemd-run --unit=vpskit-port-hop-reboot-test --on-active=5s /usr/bin/systemctl reboot
printf '%s\n' 'VPSKIT_PORTHOP_REBOOT_SCHEDULED=PASS'
