#!/usr/bin/env bash
set -euo pipefail

temporary_ruleset="$(mktemp)"
cleanup() {
  rm -f "$temporary_ruleset"
}
trap cleanup EXIT

printf '%s\n' \
  'table inet vpskit_hysteria2_port_hop {' \
  '  chain prerouting {' \
  '    type nat hook prerouting priority dstnat; policy accept;' \
  '    udp dport 20000-20010 redirect to :443 comment "VPSKit managed Hysteria2 port hop"' \
  '  }' \
  '}' > "$temporary_ruleset"

nft -c -f "$temporary_ruleset"
printf '%s\n' 'VPSKIT_PORTHOP_NFT_SYNTAX=PASS'
