#!/usr/bin/env bash
set -euo pipefail

pre_boot_id="$(cat /root/vpskit-lab31-recovery/pre-reboot-boot-id)"
current_boot_id="$(cat /proc/sys/kernel/random/boot_id)"
test "$pre_boot_id" != "$current_boot_id"
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.31'
systemctl is-enabled --quiet vpskit-xray.service
systemctl is-enabled --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-certificate-renew.timer
/usr/local/lib/vpskit/bin/xray run -test -config /etc/vpskit/generated/xray.json >/dev/null
/usr/local/lib/vpskit/bin/sing-box check -c /etc/vpskit/generated/sing-box.json
/usr/local/bin/vpskit doctor >/dev/null
python3 <<'PY'
import json
with open('/var/lib/vpskit/state.json', encoding='utf-8') as stream:
    state = json.load(stream)
assert state['schema_version'] == 4, state
assert state['profile'] == 'balanced', state
assert state['reality']['enabled'] is True, state
assert state['hysteria2']['enabled'] is True, state
PY
echo 'LAB31_REBOOT_PERSISTENCE=PASS'
