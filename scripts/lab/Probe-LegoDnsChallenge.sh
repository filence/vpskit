#!/usr/bin/env bash
set -Eeuo pipefail

lab_domain="${VPSKIT_LAB_DOMAIN:?VPSKIT_LAB_DOMAIN is required}"

archive='/root/v0.1.0-lab.16-linux-amd64.tar.gz'
parent='/root/vpskit-lego-probe'
bundle="$parent/v0.1.0-lab.16"
token_file='/root/.vpskit-cf-token'
probe_root='/var/tmp/vpskit-lego-probe'
log_file="$probe_root/lego.log"
cleanup() {
    set +e
    rm -rf -- "$parent" "$probe_root"
    rm -f -- "$archive" "$token_file"
    unset CF_DNS_API_TOKEN
}
trap cleanup EXIT HUP INT TERM

test -f "$archive"
test -f "$token_file"
install -d -m 0700 "$parent" "$probe_root"
tar -xzf "$archive" -C "$parent"
chmod 0700 "$bundle"
chmod 0755 "$bundle/lego"
CF_DNS_API_TOKEN="$(tr -d '\r\n' < "$token_file")"
export CF_DNS_API_TOKEN
test -n "$CF_DNS_API_TOKEN"
set +e
timeout --kill-after=15s 150s "$bundle/lego" run \
    --accept-tos \
    --email '' \
    --dns cloudflare \
    --dns.resolvers 1.1.1.1:53 \
    --domains "$lab_domain" \
    --path "$probe_root/state" >"$log_file" 2>&1
probe_exit=$?
set -e

python3 - "$log_file" "$token_file" "$probe_exit" <<'PY'
import pathlib
import sys

log = pathlib.Path(sys.argv[1]).read_text(errors='replace')
token = pathlib.Path(sys.argv[2]).read_text().strip()
if token:
    log = log.replace(token, '[REDACTED]')
lines = [line for line in log.splitlines() if line.strip()]
print(f'LEGO_PROBE_EXIT={sys.argv[3]}')
for line in lines[-40:]:
    print(line)
PY
