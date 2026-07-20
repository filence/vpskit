#!/usr/bin/env bash
set -euo pipefail

release_id='v0.1.0-lab.20'
release_archive="/root/${release_id}-linux-amd64.tar.gz"
release_sha256='bb9a52ebe77e7d25512d9f67e80a99dcb238758bba435cdac4370134cce32a17'
fixture_id='v0.1.0-lab.20-core-failure'
fixture_archive="/root/${fixture_id}-linux-amd64.tar.gz"
fixture_sha256='46165f3109c2fbc4364522262d0ae32b7d8639c21adc3da183e82277f5d04d9f'
lab_root='/root/vpskit-lab20-core-update'
bundle="$lab_root/$release_id"
fixture_bundle="$lab_root/$fixture_id"
evidence="$lab_root/evidence"

rm -rf -- "$lab_root"
mkdir -p -- "$evidence"
printf '%s  %s\n' "$release_sha256" "$release_archive" | sha256sum -c -
printf '%s  %s\n' "$fixture_sha256" "$fixture_archive" | sha256sum -c -
tar -xzf "$release_archive" -C "$lab_root"
tar -xzf "$fixture_archive" -C "$lab_root"
chmod 0755 "$bundle/vpskit" "$bundle/sing-box" "$bundle/lego" "$fixture_bundle/vpskit" "$fixture_bundle/lego"

before_schema="$(python3 -c 'import json; print(json.load(open("/var/lib/vpskit/state.json"))["schema_version"])')"
test "$before_schema" = '1'
sha256sum \
  /usr/local/lib/vpskit/bin/sing-box \
  /etc/vpskit/generated/sing-box.json \
  /var/lib/vpskit/certificates/hysteria2.crt \
  /var/lib/vpskit/certificates/hysteria2.key \
  /etc/vpskit/exports/* >"$evidence/before.sha256"

"$bundle/vpskit" bundle verify --dir "$bundle" | tee "$evidence/bundle-verify.json"
"$bundle/vpskit" update self --bundle-dir "$bundle" | tee "$evidence/update-self.json"
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.20'
python3 - <<'PY'
import json
state = json.load(open('/var/lib/vpskit/state.json'))
assert state['schema_version'] == 2, state
assert state['core']['version'] == '1.13.14', state['core']
assert state['core']['channel'] == 'stable', state['core']
assert state['core']['source_ref'] == 'v1.13.14', state['core']
assert state['core']['source_commit'] == '25a600db24f7680ad9806ce5427bd0ab8afe1114', state['core']
PY

/usr/local/bin/vpskit update core --bundle-dir "$bundle" --channel stable | tee "$evidence/core-already-current.json"
grep -q 'ALREADY_CURRENT' "$evidence/core-already-current.json"
/usr/local/bin/vpskit update core --bundle-dir "$bundle" --channel pinned | tee "$evidence/core-pinned.json"
grep -q 'METADATA_RECONCILED' "$evidence/core-pinned.json"
/usr/local/bin/vpskit update core --bundle-dir "$bundle" --channel stable | tee "$evidence/core-stable.json"
grep -q 'METADATA_RECONCILED' "$evidence/core-stable.json"

stable_tx="$(python3 - <<'PY'
import glob, json
records=[]
for path in glob.glob('/var/lib/vpskit/transactions/TX-*/transaction.json'):
    try:
        value=json.load(open(path))
    except Exception:
        continue
    if value.get('command') == 'update core' and value.get('status') == 'COMMITTED' and value.get('channel') == 'stable':
        records.append((value.get('committed_at',''), value['transaction_id']))
print(sorted(records)[-1][1])
PY
)"
test -n "$stable_tx"
systemctl stop vpskit-sing-box.service
! systemctl is-active --quiet vpskit-sing-box.service
/usr/local/bin/vpskit rollback "$stable_tx" --yes | tee "$evidence/rollback-while-unhealthy.json"
systemctl is-active --quiet vpskit-sing-box.service
python3 - <<'PY'
import json
state=json.load(open('/var/lib/vpskit/state.json'))
assert state['core']['channel'] == 'pinned', state['core']
PY
/usr/local/bin/vpskit update core --bundle-dir "$bundle" --channel stable | tee "$evidence/core-stable-after-rollback.json"

sha256sum \
  /usr/local/lib/vpskit/bin/sing-box \
  /var/lib/vpskit/state.json \
  /etc/vpskit/generated/sing-box.json >"$evidence/pre-failure.sha256"
set +e
/usr/local/bin/vpskit update core --bundle-dir "$fixture_bundle" --channel stable >"$evidence/core-failure.stdout" 2>"$evidence/core-failure.stderr"
failure_exit=$?
set -e
test "$failure_exit" -ne 0
grep -Eq 'service restart after core update failed|service health check after core update failed' "$evidence/core-failure.stderr"
sha256sum -c "$evidence/pre-failure.sha256"
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-certificate-renew.timer

python3 - <<'PY'
import glob, json, os
records=[]
for path in glob.glob('/var/lib/vpskit/transactions/TX-*/transaction.json'):
    try:
        value=json.load(open(path))
    except Exception:
        continue
    if value.get('command') == 'update core' and value.get('new_core_version') == '1.13.15':
        records.append((value.get('failed_at',''), path, value))
_, path, record = sorted(records)[-1]
assert record['status'] == 'ROLLED_BACK', record
assert record['previous_state_schema'] == 2, record
assert os.path.isdir('/var/lib/vpskit/backups/' + record['previous_backup_id']), record
print('FAILED_TRANSACTION=' + record['transaction_id'])
print('FAILED_TRANSACTION_STATUS=' + record['status'])
PY

python3 - <<'PY'
import json
state=json.load(open('/var/lib/vpskit/state.json'))
assert state['schema_version'] == 2, state
assert state['core']['version'] == '1.13.14', state['core']
assert state['core']['channel'] == 'stable', state['core']
PY
/usr/local/bin/vpskit status | tee "$evidence/status.json"
/usr/local/bin/vpskit doctor | tee "$evidence/doctor.json"
sha256sum \
  /usr/local/lib/vpskit/bin/sing-box \
  /etc/vpskit/generated/sing-box.json \
  /var/lib/vpskit/certificates/hysteria2.crt \
  /var/lib/vpskit/certificates/hysteria2.key \
  /etc/vpskit/exports/* >"$evidence/after.sha256"
diff -u "$evidence/before.sha256" "$evidence/after.sha256"

rm -f -- "$release_archive" "$fixture_archive"
rm -rf -- "$bundle" "$fixture_bundle"
printf 'LAB20_CORE_UPDATE=PASS\n'
printf 'SCHEMA_MIGRATION=1_TO_2_PASS\n'
printf 'ROLLBACK_WHILE_UNHEALTHY=PASS\n'
printf 'CONTROLLED_CORE_FAILURE_ROLLBACK=PASS\n'
printf 'PRODUCTION_CORE=1.13.14_STABLE\n'
