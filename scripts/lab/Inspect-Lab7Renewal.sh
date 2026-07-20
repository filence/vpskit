#!/usr/bin/env bash
set -euo pipefail

sleep 8

echo 'PROCESSES_BEGIN'
ps -eo pid=,stat=,etime=,cmd= | grep -E 'vpskit-lab-v0.1.0-lab.7-cert|/usr/local/lib/vpskit/bin/lego|vpskit cert renew' | grep -v grep || true
echo 'PROCESSES_END'
echo "SERVICE_ACTIVE=$(systemctl is-active vpskit-sing-box.service || true)"
systemctl show vpskit-sing-box.service -p ActiveState -p SubState -p ExecMainStatus -p NRestarts
ss -ltnH | grep ':443 ' || true
ss -lunH | grep ':443 ' || true
journalctl -u vpskit-sing-box.service -n 35 --no-pager
echo "CERT_SHA256=$(sha256sum /var/lib/vpskit/certificates/hysteria2.crt | awk '{print $1}')"
echo "KEY_SHA256=$(sha256sum /var/lib/vpskit/certificates/hysteria2.key | awk '{print $1}')"
echo "CONFIG_SHA256=$(sha256sum /etc/vpskit/generated/sing-box.json | awk '{print $1}')"
echo "STATE_SHA256=$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
echo 'TRANSACTIONS_BEGIN'
for path in /var/lib/vpskit/transactions/TX-*-cert-renew-*/transaction.json; do
    [ -f "$path" ] || continue
    printf '%s ' "$path"
    python3 - "$path" <<'PY'
import json
import pathlib
import sys
record = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding='utf-8'))
print(json.dumps({key: record.get(key) for key in ('transaction_id', 'status', 'command', 'error')}, ensure_ascii=False))
PY
done
echo 'TRANSACTIONS_END'
echo "STAGING_COUNT=$(find /var/lib/vpskit/transactions -mindepth 2 -maxdepth 2 -type d -name staging | wc -l)"
