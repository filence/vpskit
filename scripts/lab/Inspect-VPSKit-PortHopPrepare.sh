#!/usr/bin/env bash
set -u
printf 'VERSION=%s\n' "$(/usr/local/bin/vpskit version 2>&1)"
printf 'NFT='; command -v nft || true
printf 'UNIT='; systemctl is-active vpskit-hysteria2-port-hop.service 2>&1 || true
printf 'NFT_CONFIG='; test -f /etc/vpskit/generated/hysteria2-port-hop.nft && echo present || echo absent
printf 'UNIT_FILE='; test -f /etc/systemd/system/vpskit-hysteria2-port-hop.service && echo present || echo absent
printf 'ARCHIVE='; test -f /root/v0.2.17-lab.1-linux-amd64.tar.gz && echo present || echo absent
