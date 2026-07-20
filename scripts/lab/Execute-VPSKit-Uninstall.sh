#!/usr/bin/env bash
set -Eeuo pipefail

test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.16'
uninstall_json="$(/usr/local/bin/vpskit uninstall --yes)"
printf '%s\n' "$uninstall_json"
python3 -c 'import json,sys; p=json.loads(sys.argv[1]); assert p["status"] == "PASS", p' "$uninstall_json"

for path in \
    /usr/local/bin/vpskit \
    /usr/local/lib/vpskit \
    /etc/vpskit \
    /var/lib/vpskit \
    /var/log/vpskit \
    /etc/systemd/system/vpskit-sing-box.service \
    /etc/systemd/system/vpskit-certificate-renew.service \
    /etc/systemd/system/vpskit-certificate-renew.timer; do
    test ! -e "$path"
done

if systemctl is-active --quiet vpskit-sing-box.service; then
    echo 'UNINSTALL_SERVICE_STILL_ACTIVE' >&2
    exit 1
fi
if systemctl is-active --quiet vpskit-certificate-renew.timer; then
    echo 'UNINSTALL_TIMER_STILL_ACTIVE' >&2
    exit 1
fi
if ss -ltnH | grep -q ':443 '; then
    echo 'UNINSTALL_TCP443_STILL_LISTENING' >&2
    exit 1
fi
if ss -lunH | grep -q ':443 '; then
    echo 'UNINSTALL_UDP443_STILL_LISTENING' >&2
    exit 1
fi
! id vpskit >/dev/null 2>&1
! getent group vpskit >/dev/null 2>&1
echo 'UNINSTALL_DESTRUCTIVE_TEST=PASS'
