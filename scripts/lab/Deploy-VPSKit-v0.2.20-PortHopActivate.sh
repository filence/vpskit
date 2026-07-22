#!/usr/bin/env bash
set -euo pipefail
umask 077

version='v0.2.20-lab.1'
archive="/root/${version}-linux-amd64.tar.gz"
archive_sha256='6d5a2eee5ffa7d7ee181dba84ace800e70d8e52aa55a8ec089d878f3a4196e1d'
staging="/root/vpskit-${version}-upgrade"
rollback_binary="/var/lib/vpskit/backups/${version}-cli-upgrade/vpskit-before-${version}"
saved_nft="${staging}/previous-hysteria2-port-hop.nft"
saved_unit="${staging}/previous-vpskit-hysteria2-port-hop.service"

test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.2.17-lab.1'
test "$(sha256sum "$archive" | awk '{print $1}')" = "$archive_sha256"
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
! systemctl is-active --quiet vpskit-hysteria2-port-hop.service

state_before="$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
subscription_before="$(sha256sum /var/lib/vpskit/subscription-state.json | awk '{print $1}')"
ok=false
installed=false

cleanup() {
  exit_code=$?
  if [ "$ok" != true ]; then
    systemctl disable --now vpskit-hysteria2-port-hop.service >/dev/null 2>&1 || true
    if [ -f "$saved_nft" ]; then
      install -m 0600 "$saved_nft" /etc/vpskit/generated/hysteria2-port-hop.nft
    fi
    if [ -f "$saved_unit" ]; then
      install -m 0644 "$saved_unit" /etc/systemd/system/vpskit-hysteria2-port-hop.service
    fi
    systemctl daemon-reload >/dev/null 2>&1 || true
    if [ "$installed" = true ] && [ -f "$rollback_binary" ]; then
      install -m 0755 "$rollback_binary" /usr/local/bin/vpskit
    fi
  fi
  rm -rf -- "$staging"
  if [ "$ok" = true ]; then
    rm -f -- "$archive"
  fi
  trap - EXIT
  exit "$exit_code"
}
trap cleanup EXIT

mkdir -p -m 0700 "$staging" "$(dirname "$rollback_binary")"
cp -p /etc/vpskit/generated/hysteria2-port-hop.nft "$saved_nft"
cp -p /etc/systemd/system/vpskit-hysteria2-port-hop.service "$saved_unit"
tar -xzf "$archive" -C "$staging"
"$staging/$version/vpskit" bundle verify --dir "$staging/$version" >/dev/null
install -m 0700 /usr/local/bin/vpskit "$rollback_binary"
install -m 0755 "$staging/$version/vpskit" /usr/local/bin/vpskit
installed=true
test "$(/usr/local/bin/vpskit version)" = "vpskit $version"

/usr/local/bin/vpskit hysteria2 port-hop prepare --range 20000-20010 --backend-port 443 --yes >/root/vpskit-port-hop-prepare.json
nft -c -f /etc/vpskit/generated/hysteria2-port-hop.nft
/usr/local/bin/vpskit hysteria2 port-hop activate --yes >/root/vpskit-port-hop-activate.json
/usr/local/bin/vpskit hysteria2 port-hop status >/root/vpskit-port-hop-status.json

systemctl is-active --quiet vpskit-hysteria2-port-hop.service
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
nft list table inet vpskit_hysteria2_port_hop >/root/vpskit-port-hop-nft.txt
test "$state_before" = "$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
test "$subscription_before" = "$(sha256sum /var/lib/vpskit/subscription-state.json | awk '{print $1}')"

ok=true
printf 'PORTHOP_ACTIVATE_DEPLOY=PASS version=%s\n' "$version"
