#!/usr/bin/env bash
set -euo pipefail

/usr/local/bin/vpskit security fail2ban remove --yes
test ! -e /etc/fail2ban/jail.d/vpskit-sshd.conf
/usr/local/bin/vpskit security fail2ban status
/usr/local/bin/vpskit security fail2ban apply --yes
/usr/local/bin/vpskit security fail2ban status
fail2ban-client status sshd
