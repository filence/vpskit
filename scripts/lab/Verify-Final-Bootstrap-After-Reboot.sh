#!/usr/bin/env bash
set -Eeuo pipefail

readonly recovery_root='/root/vpskit-final-bootstrap-lab33'
readonly loopback='/root/vpskit-final-bootstrap-lab33-loopback.sh'

test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.33'
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-certificate-renew.timer
/usr/local/bin/vpskit doctor >/dev/null
/usr/local/bin/vpskit cert status >/dev/null
bash "$loopback" >"$recovery_root/post-reboot-loopback.log" 2>&1
grep -q '^REMOTE_LOOPBACK_CLIENTS=PASS$' "$recovery_root/post-reboot-loopback.log"
test -s "$recovery_root/new-client.zip"
test "$(stat -c '%a' "$recovery_root/new-client.zip")" = '600'
printf 'FINAL_BOOTSTRAP_REBOOT_SERVICES=PASS\n'
printf 'FINAL_BOOTSTRAP_REBOOT_LOOPBACK=PASS\n'
printf 'FINAL_BOOTSTRAP_POST_REBOOT=PASS\n'
