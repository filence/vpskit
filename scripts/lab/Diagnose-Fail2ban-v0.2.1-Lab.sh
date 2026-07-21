#!/usr/bin/env bash
set -euo pipefail

systemctl status fail2ban.service --no-pager -n 80 || true
journalctl -u fail2ban.service -n 120 --no-pager || true
printf '\n--- jail ---\n'
sed -n '1,160p' /etc/fail2ban/jail.d/vpskit-sshd.conf || true
printf '\n--- client status ---\n'
fail2ban-client status || true
fail2ban-client status vpskit-sshd || true
printf '\n--- effective configuration ---\n'
fail2ban-client -d 2>&1 | tail -n 160 || true
