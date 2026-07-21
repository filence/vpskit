#!/usr/bin/env bash
set -euo pipefail

/usr/local/bin/vpskit version
/usr/local/bin/vpskit security fail2ban plan
/usr/local/bin/vpskit security fail2ban apply --yes
/usr/local/bin/vpskit security fail2ban status
fail2ban-client status sshd
systemctl is-enabled fail2ban.service
/usr/local/bin/vpskit doctor
/usr/local/bin/vpskit status
/usr/local/bin/vpskit system inspect
