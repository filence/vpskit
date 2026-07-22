#!/usr/bin/env bash
set -euo pipefail
umask 077
test ! -x /usr/sbin/nft
apt-get install -y --no-install-recommends nftables
test -x /usr/sbin/nft
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
printf 'NFTABLES_PREREQUISITE=PASS version=%s\n' "$(nft --version)"
