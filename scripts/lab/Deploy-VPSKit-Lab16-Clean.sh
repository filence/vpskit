#!/usr/bin/env bash
set -Eeuo pipefail

lab_domain="${VPSKIT_LAB_DOMAIN:?VPSKIT_LAB_DOMAIN is required}"

archive='/root/v0.1.0-lab.16-linux-amd64.tar.gz'
archive_sha256='6ce50454845f4fdf25db6925fb5fdba63d37a871040594479dad8e8fbfeec456'
release_parent='/root/vpskit-lab-v0.1.0-lab.16-clean'
bundle="$release_parent/v0.1.0-lab.16"
token_file='/root/.vpskit-cf-token'

cleanup() {
    set +e
    unset VPSKIT_CF_DNS_API_TOKEN || true
    rm -f -- "$token_file" "$archive"
    rm -rf -- "$release_parent"
}
trap cleanup EXIT HUP INT TERM

test -f "$archive"
test -f "$token_file"
test ! -e "$release_parent"
printf '%s  %s\n' "$archive_sha256" "$archive" | sha256sum -c -
install -d -m 0700 "$release_parent"
tar -xzf "$archive" -C "$release_parent"
chmod 0700 "$bundle"
chmod 0755 "$bundle/vpskit" "$bundle/sing-box" "$bundle/lego"
chmod 0644 "$bundle/release-manifest.json" "$bundle/release-manifest.sig"

VPSKIT_CF_DNS_API_TOKEN="$(tr -d '\r\n' < "$token_file")"
export VPSKIT_CF_DNS_API_TOKEN
test -n "$VPSKIT_CF_DNS_API_TOKEN"
"$bundle/vpskit" bundle verify --dir "$bundle"
"$bundle/vpskit" preflight \
    --tcp-port 443 \
    --udp-port 443 \
    --reality-server-name www.amazon.com \
    --sing-box "$bundle/sing-box"
timeout --kill-after=20s 240s "$bundle/vpskit" install balanced \
    --bundle-dir "$bundle" \
    --domain "$lab_domain" \
    --connect-host "$lab_domain" \
    --reality-server-name www.amazon.com \
    --tcp-port 443 \
    --udp-port 443
"$bundle/vpskit" status
"$bundle/vpskit" doctor
systemctl is-enabled --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-enabled --quiet vpskit-certificate-renew.timer
systemctl is-active --quiet vpskit-certificate-renew.timer
ss -ltnH | grep -q ':443 '
ss -lunH | grep -q ':443 '
printf 'CLEAN_DEPLOY_RESULT status=PASS\n'
