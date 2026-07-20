#!/usr/bin/env bash
set -Eeuo pipefail

lab_domain="${VPSKIT_LAB_DOMAIN:?VPSKIT_LAB_DOMAIN is required}"

archive='/root/v0.1.0-lab.16-linux-amd64.tar.gz'
release_parent='/root/vpskit-lab-v0.1.0-lab.16-debug'
bundle="$release_parent/v0.1.0-lab.16"
token_file='/root/.vpskit-cf-token'
trap 'unset VPSKIT_CF_DNS_API_TOKEN || true' EXIT HUP INT TERM

echo PHASE=checks
test -f "$archive"
test -f "$token_file"
test ! -e "$release_parent"
echo PHASE=extract
install -d -m 0700 "$release_parent"
tar -xzf "$archive" -C "$release_parent"
chmod 0700 "$bundle"
chmod 0755 "$bundle/vpskit" "$bundle/sing-box" "$bundle/lego"
chmod 0644 "$bundle/release-manifest.json" "$bundle/release-manifest.sig"
echo PHASE=verify
"$bundle/vpskit" bundle verify --dir "$bundle"
echo PHASE=preflight
"$bundle/vpskit" preflight --tcp-port 443 --udp-port 443 --reality-server-name www.amazon.com --sing-box "$bundle/sing-box"
echo PHASE=token
VPSKIT_CF_DNS_API_TOKEN="$(tr -d '\r\n' < "$token_file")"
export VPSKIT_CF_DNS_API_TOKEN
if [ -z "$VPSKIT_CF_DNS_API_TOKEN" ]; then echo TOKEN_EMPTY >&2; exit 1; fi
echo PHASE=install
"$bundle/vpskit" install balanced --bundle-dir "$bundle" --domain "$lab_domain" --connect-host "$lab_domain" --reality-server-name www.amazon.com --tcp-port 443 --udp-port 443
echo PHASE=complete
