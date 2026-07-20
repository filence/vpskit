#!/usr/bin/env bash
set -euo pipefail

python3 <<'PY'
import json
with open('/var/lib/vpskit/state.json', encoding='utf-8') as stream:
    state = json.load(stream)
print('PROFILE=' + state['profile'])
print('REALITY_ENABLED=' + str(state['reality']['enabled']).lower())
print('HYSTERIA2_ENABLED=' + str(state['hysteria2']['enabled']).lower())
PY
printf 'XRAY_ACTIVE=%s\n' "$(systemctl is-active vpskit-xray.service || true)"
printf 'SING_BOX_ACTIVE=%s\n' "$(systemctl is-active vpskit-sing-box.service || true)"
printf 'RENEW_TIMER_ACTIVE=%s\n' "$(systemctl is-active vpskit-certificate-renew.timer || true)"
find /var/lib/vpskit/backups -mindepth 1 -maxdepth 1 -type d -name 'BK-*' -printf '%f\n' | sort | tail -n 3 | sed 's/^/BACKUP=/'
