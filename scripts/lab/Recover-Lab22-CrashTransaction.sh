#!/usr/bin/env bash
set -euo pipefail

transaction_id='TX-20260718-090438-update-core-d29d02'
transaction_root="/var/lib/vpskit/transactions/$transaction_id"
record="$transaction_root/transaction.json"
backup="$transaction_root/backup"

python3 - "$record" <<'PY'
import json, sys
value=json.load(open(sys.argv[1]))
assert value['status'] == 'IN_PROGRESS', value
assert value['command'] == 'update core', value
assert value['new_core_version'] == '1.13.16', value
PY
for file in sing-box state.json ownership.json versions.lock; do
    test -s "$backup/$file"
done

install -m 0755 -o root -g root "$backup/sing-box" /usr/local/lib/vpskit/bin/sing-box
install -m 0600 -o root -g root "$backup/state.json" /var/lib/vpskit/state.json
install -m 0600 -o root -g root "$backup/ownership.json" /var/lib/vpskit/ownership.json
install -m 0644 -o root -g root "$backup/versions.lock" /etc/vpskit/versions.lock
systemctl restart vpskit-sing-box.service
systemctl is-active --quiet vpskit-sing-box.service
test "$(/usr/local/lib/vpskit/bin/sing-box version | head -n1)" = 'sing-box version 1.13.14'

python3 - "$record" <<'PY'
import datetime, json, sys
path=sys.argv[1]
value=json.load(open(path))
value['status']='RECOVERED'
value['recovered_at']=datetime.datetime.now(datetime.timezone.utc).isoformat().replace('+00:00','Z')
value['recovery_source']='transaction-local-backup'
with open(path, 'w') as handle:
    json.dump(value, handle, indent=2)
    handle.write('\n')
PY

/usr/local/bin/vpskit doctor
printf 'LAB22_EMERGENCY_LOCAL_RECOVERY=PASS\n'
