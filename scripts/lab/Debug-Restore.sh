#!/usr/bin/env bash
set -u
backup_id='BK-20260718-023355-81811e'
echo "BACKUP=$backup_id"
ls -l "/var/lib/vpskit/backups/$backup_id/backup.json"
/usr/local/bin/vpskit version
set +e
/usr/local/bin/vpskit restore "$backup_id" --yes
exit_code=$?
set -e
echo "RESTORE_EXIT=$exit_code"
exit 0
