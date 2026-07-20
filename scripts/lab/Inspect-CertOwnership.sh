#!/usr/bin/env bash
set -euo pipefail
stat -c '%a %U:%G %n' /var/lib/vpskit/certificates/hysteria2.crt /var/lib/vpskit/certificates/hysteria2.key
python3 - <<'PY'
import json
import pathlib

for path in sorted(pathlib.Path('/var/lib/vpskit/transactions').glob('TX-*-cert-renew-*/transaction.json')):
    print(path)
    print(path.read_text(encoding='utf-8'))
PY
