#!/usr/bin/env bash
set -euo pipefail

test "$(id -u)" = 0
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.29'
systemctl is-active --quiet vpskit-sing-box.service
if systemctl is-active --quiet vpskit-xray.service; then
    exit 1
fi
/usr/local/lib/vpskit/bin/sing-box check -c /etc/vpskit/generated/sing-box.json >/dev/null
/usr/local/bin/vpskit status

test "$(python3 -c 'import json; print(json.load(open("/var/lib/vpskit/state.json"))["schema_version"])')" = 3
test -r /var/lib/vpskit/secrets/cloudflare.env
test -r /var/lib/vpskit/secrets/acme.env
test "$(stat -c '%a' /var/lib/vpskit/secrets/cloudflare.env)" = 600
test "$(stat -c '%a' /var/lib/vpskit/secrets/acme.env)" = 600

ss -ltnH | grep -q ':443 '
ss -lunH | grep -q ':443 '

if systemctl list-unit-files --no-legend | grep -q '^vpskit-xray-compat\.service'; then
    exit 1
fi
test ! -e /root/vpskit-xray-compat-runtime
test ! -e /root/vpskit-xray-compat-xray

timedatectl show --property=NTPSynchronized --value
df -h / /var/lib/vpskit
free -m
printf 'LAB30_PREFLIGHT=PASS\n'
