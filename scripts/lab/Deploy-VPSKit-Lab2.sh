#!/usr/bin/env bash
set -eu

lab_domain="${VPSKIT_LAB_DOMAIN:?VPSKIT_LAB_DOMAIN is required}"

archive='/root/v0.1.0-lab.2-linux-amd64.tar.gz'
archive_sha256='8483a0e266247f0524158011752c630c3260ba32b401f56668f26c6d1c4e489f'
release_parent='/root/vpskit-lab-v0.1.0-lab.2'
bundle="${release_parent}/v0.1.0-lab.2"
token_file='/root/.vpskit-cf-token'

cleanup_token() {
    unset VPSKIT_CF_DNS_API_TOKEN || true
    rm -f -- "$token_file"
}
trap cleanup_token EXIT HUP INT TERM

if [ ! -f "$archive" ]; then
    echo 'DEPLOY_FAIL reason=archive_missing' >&2
    exit 1
fi
if [ ! -f "$token_file" ]; then
    echo 'DEPLOY_FAIL reason=token_file_missing' >&2
    exit 1
fi
if [ -e "$release_parent" ]; then
    echo 'DEPLOY_FAIL reason=release_staging_already_exists' >&2
    exit 1
fi

printf '%s  %s\n' "$archive_sha256" "$archive" | sha256sum -c -
install -d -m 0700 "$release_parent"
tar -xzf "$archive" -C "$release_parent"
chmod 0700 "$bundle"
chmod 0755 "$bundle/vpskit" "$bundle/sing-box" "$bundle/lego"
chmod 0644 "$bundle/release-manifest.json" "$bundle/release-manifest.sig"

VPSKIT_CF_DNS_API_TOKEN="$(tr -d '\r\n' < "$token_file")"
export VPSKIT_CF_DNS_API_TOKEN
if [ -z "$VPSKIT_CF_DNS_API_TOKEN" ]; then
    echo 'DEPLOY_FAIL reason=empty_token' >&2
    exit 1
fi

"$bundle/vpskit" bundle verify --dir "$bundle"
"$bundle/vpskit" preflight \
    --tcp-port 443 \
    --udp-port 443 \
    --reality-server-name www.microsoft.com
"$bundle/vpskit" install balanced \
    --bundle-dir "$bundle" \
    --domain "$lab_domain" \
    --connect-host "$lab_domain" \
    --reality-server-name www.microsoft.com \
    --tcp-port 443 \
    --udp-port 443
"$bundle/vpskit" status
"$bundle/vpskit" doctor
printf 'DEPLOY_RESULT status=PASS\n'
