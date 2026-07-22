#!/usr/bin/env bash
set -euo pipefail

readonly target_bytes=2097152
readonly sysctl_file='/etc/sysctl.d/70-vpskit-hysteria2-udp-buffer.conf'
readonly ownership_file='/var/lib/vpskit/hysteria2/udp-buffer.json'

/usr/local/bin/vpskit hysteria2 udp-buffer status >/root/vpskit-hysteria2-udp-buffer-final-status.json
/usr/local/bin/vpskit orphan scan >/root/vpskit-hysteria2-udp-buffer-orphan.json
python3 - <<'PY'
import json
from pathlib import Path

status = json.loads(Path('/root/vpskit-hysteria2-udp-buffer-final-status.json').read_text(encoding='utf-8'))
orphan = json.loads(Path('/root/vpskit-hysteria2-udp-buffer-orphan.json').read_text(encoding='utf-8'))
detail = status['detail']
assert status['status'] == 'PASS', status
assert detail['managed'] is True, detail
assert detail['socket_buffers']['receive_bytes'] >= 2097152, detail
assert detail['socket_buffers']['send_bytes'] >= 2097152, detail
assert orphan['status'] == 'PASS', orphan
assert all(item['path'] != '/var/lib/vpskit/hysteria2' for item in orphan['detail']['orphan_candidates']), orphan
PY
test -f "$sysctl_file"
test -f "$ownership_file"
for key in net.core.rmem_default net.core.wmem_default net.core.rmem_max net.core.wmem_max; do
  test "$(sysctl -n "$key")" = "$target_bytes"
done
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
printf 'HYSTERIA2_UDP_BUFFER_READBACK=PASS version=%s state=%s subscription=%s\n' \
  "$(/usr/local/bin/vpskit version | awk '{print $2}')" \
  "$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')" \
  "$(sha256sum /var/lib/vpskit/subscription-state.json | awk '{print $1}')"
