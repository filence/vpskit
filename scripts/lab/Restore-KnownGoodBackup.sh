#!/usr/bin/env bash
set -Eeuo pipefail

backup_id='BK-20260718-023355-81811'
test -f "/var/lib/vpskit/backups/$backup_id/backup.json"
/usr/local/bin/vpskit restore "$backup_id" --yes
test "$(sha256sum /etc/vpskit/generated/sing-box.json | awk '{print $1}')" = 'd4e4d2e4528f6471746f34a4222745188097cdb5bd7ee46bf040d930261c844f'
test "$(stat -c '%U:%G %a' /etc/vpskit/generated/sing-box.json)" = 'root:vpskit 640'
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-certificate-renew.timer
ss -ltnH | grep -q ':443 '
ss -lunH | grep -q ':443 '
echo 'KNOWN_GOOD_BACKUP_RESTORE=PASS'
