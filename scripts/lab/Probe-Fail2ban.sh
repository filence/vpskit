#!/usr/bin/env bash
set -euo pipefail

if command -v fail2ban-client >/dev/null 2>&1; then
  fail2ban-client version
  systemctl is-active fail2ban.service || true
  fail2ban-client status || true
else
  echo 'fail2ban=NOT_INSTALLED'
  apt-cache policy fail2ban | sed -n '1,12p'
fi

systemctl is-active systemd-journald.service
systemctl is-active ssh.service || systemctl is-active sshd.service || true
ss -ltnH | grep -E ':(22|[0-9]+) ' | head -n 20
journalctl -u ssh.service -n 3 --no-pager 2>/dev/null || journalctl -u sshd.service -n 3 --no-pager 2>/dev/null || true
