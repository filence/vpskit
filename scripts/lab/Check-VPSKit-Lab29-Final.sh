#!/usr/bin/env bash
set -Eeuo pipefail

test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.29'
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-certificate-renew.timer
/usr/local/lib/vpskit/bin/sing-box check -c /etc/vpskit/generated/sing-box.json

status_output="$(/usr/local/bin/vpskit status)"
doctor_output="$(/usr/local/bin/vpskit doctor)"
recover_output="$(/usr/local/bin/vpskit recover)"
orphan_output="$(/usr/local/bin/vpskit orphan scan)"
certificate_output="$(/usr/local/bin/vpskit cert status)"

python3 - "$status_output" "$doctor_output" "$recover_output" "$orphan_output" "$certificate_output" <<'PY'
import json
import os
import stat
import sys
from pathlib import Path

status, doctor, recover, orphan, certificate = map(json.loads, sys.argv[1:])
for result in (status, doctor, recover, orphan, certificate):
    assert result['status'] == 'PASS', result
detail = status['detail']
assert detail['state_schema'] == 3
assert detail['profile'] == 'balanced'
assert detail['service_active'] is True
assert detail['reality_enabled'] is True
assert detail['hysteria2_enabled'] is True
assert detail['tcp_listener'] is True
assert detail['udp_listener'] is True
assert detail['config_hash_matches'] is True
assert detail['incomplete_transactions'] == []
assert detail['incomplete_core_transactions'] == []
assert recover['detail']['result'] == 'NOT_NEEDED'
assert (orphan.get('detail') or {}).get('needs_review') in (False, None)
assert certificate['detail']['renewal_timer_active'] is True
assert stat.S_IMODE(os.stat('/var/lib/vpskit/certificates').st_mode) == 0o750
assert stat.S_IMODE(os.stat('/var/lib/vpskit/certificates/hysteria2.crt').st_mode) == 0o640
for path in (
    '/etc/vpskit/exports/mihomo.yaml',
    '/etc/vpskit/exports/sing-box-reality.json',
    '/etc/vpskit/exports/sing-box-hysteria2.json',
    '/etc/vpskit/exports/share-links.txt',
):
    assert Path(path).is_file(), path
print('LAB29_FINAL_STATE=PASS')
PY

runuser -u vpskit -- test -r /etc/vpskit/generated/sing-box.json
runuser -u vpskit -- test -r /var/lib/vpskit/certificates/hysteria2.crt
runuser -u vpskit -- test -r /var/lib/vpskit/certificates/hysteria2.key

/usr/local/bin/vpskit export --format qr >/tmp/vpskit-lab29-final-qr.txt
grep -Fq 'VPSKit REALITY QR' /tmp/vpskit-lab29-final-qr.txt
grep -Fq 'VPSKit HYSTERIA2 QR' /tmp/vpskit-lab29-final-qr.txt
! grep -Eq 'vless://|hysteria2://' /tmp/vpskit-lab29-final-qr.txt
rm -f -- /tmp/vpskit-lab29-final-qr.txt

for path in \
    /root/vpskit-lab25 \
    /root/vpskit-lab26 \
    /root/vpskit-lab27 \
    /root/vpskit-lab28 \
    /root/vpskit-lab29 \
    /root/vpskit-lab25-systemctl-shim \
    /root/vpskit-lab27-reality-clean-test \
    /root/vpskit-lab29-reality-clean-test; do
    test ! -e "$path"
done

printf 'LAB29_FINAL_QR=PASS\n'
printf 'LAB29_FINAL=PASS\n'
