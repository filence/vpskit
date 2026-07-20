#!/usr/bin/env bash
set -Eeuo pipefail

recovery_root='/root/vpskit-lab31-recovery'
obsolete_archive='/root/v0.1.0-lab.30-linux-amd64.tar.gz'
managed_backup='/var/lib/vpskit/backups/BK-20260720-011943-pre-update-8c36e4'

test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.31'
test -d "$managed_backup"
test -d "$recovery_root"
test -f "$obsolete_archive"

resolved_recovery="$(readlink -f -- "$recovery_root")"
resolved_archive="$(readlink -f -- "$obsolete_archive")"
test "$resolved_recovery" = '/root/vpskit-lab31-recovery'
test "$resolved_archive" = '/root/v0.1.0-lab.30-linux-amd64.tar.gz'

recovery_bytes="$(du -sb -- "$resolved_recovery" | awk '{print $1}')"
archive_bytes="$(stat -c '%s' -- "$resolved_archive")"

rm -rf -- "$resolved_recovery"
rm -f -- "$resolved_archive"

test ! -e "$recovery_root"
test ! -e "$obsolete_archive"
test -d "$managed_backup"
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-certificate-renew.timer
/usr/local/bin/vpskit doctor >/dev/null

printf 'REMOTE_CLEANUP=PASS recovery_bytes=%s archive_bytes=%s\n' "$recovery_bytes" "$archive_bytes"
echo 'MANAGED_ROLLBACK_BACKUP=PRESERVED'
