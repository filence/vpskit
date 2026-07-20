#!/usr/bin/env bash
set -euo pipefail
python3 <<'PY'
import json
import pathlib
root = pathlib.Path('/var/lib/vpskit/transactions')
for path in sorted(root.glob('TX-*/transaction.json'))[-12:]:
    print(path)
    print(json.loads(path.read_text(encoding='utf-8')))
PY
systemctl is-active vpskit-sing-box.service || true
systemctl is-active vpskit-certificate-renew.timer || true
sha256sum /etc/vpskit/generated/sing-box.json
stat -c '%a %U:%G %n' /etc/vpskit/generated/sing-box.json /var/lib/vpskit/state.json
