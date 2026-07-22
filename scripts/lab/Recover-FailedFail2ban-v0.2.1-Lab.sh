#!/usr/bin/env bash
set -euo pipefail

path=/etc/fail2ban/jail.d/vpskit-sshd.conf
if [ -f "$path" ] && grep -qx '\[vpskit-sshd\]' "$path"; then
  rm -f -- "$path"
fi
systemctl restart fail2ban.service
for _ in $(seq 1 20); do
  if fail2ban-client status sshd >/dev/null 2>&1; then
    fail2ban-client status sshd
    exit 0
  fi
  sleep 1
done
systemctl status fail2ban.service --no-pager -n 80
exit 1
