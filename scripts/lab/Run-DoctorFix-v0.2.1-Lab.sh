#!/usr/bin/env bash
set -euo pipefail

/usr/local/bin/vpskit version
/usr/local/bin/vpskit doctor --fix
/usr/local/bin/vpskit doctor
/usr/local/bin/vpskit status
systemctl is-enabled vpskit-xray.service
systemctl is-enabled vpskit-sing-box.service
systemctl is-enabled vpskit-certificate-renew.timer
