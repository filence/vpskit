#!/usr/bin/env bash
set -euo pipefail

echo 'CLOSEOUT_AUDIT=START'
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.31'
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-certificate-renew.timer
/usr/local/lib/vpskit/bin/xray run -test -config /etc/vpskit/generated/xray.json >/dev/null
/usr/local/lib/vpskit/bin/sing-box check -c /etc/vpskit/generated/sing-box.json

doctor_json="$(/usr/local/bin/vpskit doctor)"
python3 - "$doctor_json" <<'PY'
import json
import sys

value = json.loads(sys.argv[1])
assert value['status'] == 'PASS', value
detail = value['detail']
assert detail['profile'] == 'balanced', detail
assert detail['state_schema'] == 4, detail
assert detail['config_hash_matches'] is True, detail
assert detail['reality_config_hash_matches'] is True, detail
assert detail['service_state_matches'] is True, detail
assert detail['tcp_listener'] is True, detail
assert detail['udp_listener'] is True, detail
assert not detail['incomplete_transactions'], detail
assert not detail['incomplete_core_transactions'], detail
print('DOCTOR_FINAL=PASS')
print('CERTIFICATE_NOT_AFTER=' + detail['certificate_not_after'])
PY

printf 'XRAY_NRESTARTS=%s\n' "$(systemctl show vpskit-xray.service -p NRestarts --value)"
printf 'SING_BOX_NRESTARTS=%s\n' "$(systemctl show vpskit-sing-box.service -p NRestarts --value)"
ps -C xray -C sing-box -o comm=,rss=,%cpu= | awk '{printf "PROCESS=%s RSS_KB=%s CPU=%s\n", $1, $2, $3}'

echo 'ACTIVE_SECRET_FILES=START'
find /var/lib/vpskit/secrets -maxdepth 1 -type f -printf '%p|mode=%m|bytes=%s\n' | sort
echo 'ACTIVE_SECRET_FILES=END'

echo 'BACKUP_CANDIDATES=START'
if [ -d /var/lib/vpskit/backups ]; then
  find /var/lib/vpskit/backups -mindepth 1 -maxdepth 1 -type d -name 'BK-*' -printf '%p\n' | sort | while IFS= read -r path; do
    printf '%s|bytes=%s\n' "$path" "$(du -sb "$path" | awk '{print $1}')"
  done
fi
echo 'BACKUP_CANDIDATES=END'

echo 'RECOVERY_CANDIDATES=START'
if [ -d /root/vpskit-lab31-recovery ]; then
  printf '/root/vpskit-lab31-recovery|bytes=%s\n' "$(du -sb /root/vpskit-lab31-recovery | awk '{print $1}')"
  find /root/vpskit-lab31-recovery -mindepth 1 -maxdepth 1 -printf '%p|type=%y|mode=%m|bytes=%s\n' | sort
fi
echo 'RECOVERY_CANDIDATES=END'

echo 'ROOT_LAB_ARTIFACTS=START'
find /root -mindepth 1 -maxdepth 1 \( -name 'vpskit-lab31-*' -o -name 'v0.1.0-lab.*-linux-amd64.tar.gz' \) -printf '%p|type=%y|bytes=%s\n' | sort
echo 'ROOT_LAB_ARTIFACTS=END'

echo 'CLOSEOUT_AUDIT=PASS'
