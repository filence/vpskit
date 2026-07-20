#!/usr/bin/env bash
set -euo pipefail

removed_staging=0
removed_backup=0
for record in /var/lib/vpskit/transactions/TX-*/transaction.json; do
    test -f "$record" || continue
    status="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("status",""))' "$record")"
    test "$status" = 'RECOVERED' || continue
    directory="$(dirname "$record")"
    if [ -d "$directory/staging" ]; then
        rm -rf -- "$directory/staging"
        removed_staging=$((removed_staging + 1))
    fi
    source="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("recovery_source",""))' "$record")"
    if [ "$source" = 'persistent-backup' ] && [ -d "$directory/backup" ]; then
        rm -rf -- "$directory/backup"
        removed_backup=$((removed_backup + 1))
    fi
done
test "$(find /var/lib/vpskit/transactions -type d -name staging | wc -l)" -eq 0
printf 'RECOVERED_STAGING_CLEANUP=PASS staging=%s backup=%s\n' "$removed_staging" "$removed_backup"
