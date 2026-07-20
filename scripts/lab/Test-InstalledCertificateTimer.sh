#!/usr/bin/env bash
set -Eeuo pipefail

systemctl is-enabled --quiet vpskit-certificate-renew.timer
systemctl is-active --quiet vpskit-certificate-renew.timer
systemctl start --wait vpskit-certificate-renew.service
test "$(systemctl show -p Result --value vpskit-certificate-renew.service)" = 'success'
test "$(systemctl show -p ExecMainStatus --value vpskit-certificate-renew.service)" = '0'
status_json="$(/usr/local/bin/vpskit cert status)"
python3 -c 'import json,sys; p=json.loads(sys.argv[1]); assert p["status"] == "PASS", p; assert p["detail"]["renewal_timer_active"] is True, p; print("INSTALLED_CERT_TIMER=PASS")' "$status_json"
echo 'CERTIFICATE_RENEWAL_SERVICE_EXECUTION=PASS'
