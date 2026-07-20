#!/usr/bin/env bash
set -Eeuo pipefail
find /var/lib/vpskit/transactions -maxdepth 3 -type d -print | sort
for record in /var/lib/vpskit/transactions/*/transaction.json; do
    test -f "$record" || continue
    echo "RECORD=$record"
    grep -E '"(status|transaction_id|error)"' "$record" || true
done
