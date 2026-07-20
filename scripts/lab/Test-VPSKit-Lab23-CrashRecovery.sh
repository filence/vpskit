#!/usr/bin/env bash
set -euo pipefail

release_id='v0.1.0-lab.23'
release_archive="/root/${release_id}-linux-amd64.tar.gz"
release_sha256='e8c1bd114dd68378848cff78382a6ccac10a7b0e91ad0aaa4d53bfa5cb12821d'
crash_id='v0.1.0-lab.23-core-crash'
crash_archive="/root/${crash_id}-linux-amd64.tar.gz"
crash_sha256='051b97bd6049ec0f9f364948a6d3dbf4b77720512ea234c390353d3a7c641c9b'
lab_root='/root/vpskit-lab23-crash-recovery'
bundle="$lab_root/$release_id"
crash_bundle="$lab_root/$crash_id"
evidence="$lab_root/evidence"
updater_pid=''
child_pids=''

cleanup_processes() {
    if [ -n "$updater_pid" ]; then
        kill -KILL "$updater_pid" 2>/dev/null || true
    fi
    for pid in $child_pids; do
        kill -KILL "$pid" 2>/dev/null || true
    done
}
trap cleanup_processes EXIT HUP INT TERM

rm -rf -- "$lab_root"
mkdir -p -- "$evidence"
printf '%s  %s\n' "$release_sha256" "$release_archive" | sha256sum -c -
printf '%s  %s\n' "$crash_sha256" "$crash_archive" | sha256sum -c -
tar -xzf "$release_archive" -C "$lab_root"
tar -xzf "$crash_archive" -C "$lab_root"
chmod 0755 "$bundle/vpskit" "$bundle/sing-box" "$bundle/lego" "$crash_bundle/vpskit" "$crash_bundle/sing-box" "$crash_bundle/lego"

"$bundle/vpskit" bundle verify --dir "$bundle" | tee "$evidence/release-verify.json"
"$bundle/vpskit" bundle verify --dir "$crash_bundle" | tee "$evidence/crash-fixture-verify.json"
"$bundle/vpskit" update self --bundle-dir "$bundle" | tee "$evidence/update-self.json"
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.23'

sha256sum \
  /usr/local/lib/vpskit/bin/sing-box \
  /etc/vpskit/versions.lock \
  /etc/vpskit/generated/sing-box.json >"$evidence/before-crash.sha256"
python3 - "$evidence/state-before.json" "$evidence/ownership-before.json" <<'PY'
import json, sys
for source, destination in [('/var/lib/vpskit/state.json', sys.argv[1]), ('/var/lib/vpskit/ownership.json', sys.argv[2])]:
    value=json.load(open(source))
    value.pop('transaction_id', None)
    with open(destination, 'w') as handle:
        json.dump(value, handle, sort_keys=True)
PY

/usr/local/bin/vpskit update core --bundle-dir "$crash_bundle" --channel stable >"$evidence/crash-update.stdout" 2>"$evidence/crash-update.stderr" &
updater_pid=$!
crash_tx=''
attempt=0
while [ "$attempt" -lt 100 ]; do
    crash_tx="$(python3 - <<'PY'
import glob, json
values=[]
for path in glob.glob('/var/lib/vpskit/transactions/TX-*/transaction.json'):
    try:
        record=json.load(open(path))
    except Exception:
        continue
    if record.get('command') == 'update core' and record.get('status') == 'IN_PROGRESS' and record.get('phase') == 'APPLYING' and record.get('new_core_version') == '1.13.17':
        values.append(record['transaction_id'])
print(sorted(values)[-1] if values else '')
PY
)"
    state_version="$(python3 -c 'import json; print(json.load(open("/var/lib/vpskit/state.json"))["core"]["version"])')"
    if [ -n "$crash_tx" ] && [ "$state_version" = '1.13.17' ]; then
        break
    fi
    kill -0 "$updater_pid" 2>/dev/null
    attempt=$((attempt + 1))
    sleep 0.2
done
test -n "$crash_tx"
test "$state_version" = '1.13.17'
child_pids="$(pgrep -P "$updater_pid" || true)"
kill -KILL "$updater_pid"
set +e
wait "$updater_pid"
killed_exit=$?
set -e
test "$killed_exit" -ne 0
updater_pid=''
for pid in $child_pids; do
    kill -KILL "$pid" 2>/dev/null || true
done
child_pids=''

/usr/local/bin/vpskit status | tee "$evidence/status-incomplete.json"
python3 - "$evidence/status-incomplete.json" "$crash_tx" <<'PY'
import json, sys
value=json.load(open(sys.argv[1]))
assert sys.argv[2] in value['detail']['incomplete_core_transactions'], value
PY

/usr/local/bin/vpskit update core --bundle-dir "$bundle" --channel stable | tee "$evidence/automatic-recovery.json"
grep -q 'ALREADY_CURRENT' "$evidence/automatic-recovery.json"
sha256sum -c "$evidence/before-crash.sha256"
python3 - "$evidence/state-before.json" "$evidence/ownership-before.json" <<'PY'
import json, sys
for current, expected in [('/var/lib/vpskit/state.json', sys.argv[1]), ('/var/lib/vpskit/ownership.json', sys.argv[2])]:
    value=json.load(open(current))
    value.pop('transaction_id', None)
    assert value == json.load(open(expected)), current
PY
python3 - "$crash_tx" <<'PY'
import json, sys
path=f'/var/lib/vpskit/transactions/{sys.argv[1]}/transaction.json'
record=json.load(open(path))
assert record['status'] == 'RECOVERED', record
assert record['recovery_backup_id'] == record['previous_backup_id'], record
print('CRASH_TRANSACTION=' + record['transaction_id'])
print('CRASH_TRANSACTION_STATUS=' + record['status'])
PY

/usr/local/bin/vpskit recover | tee "$evidence/recover-not-needed.json"
grep -q 'NOT_NEEDED' "$evidence/recover-not-needed.json"
/usr/local/bin/vpskit orphan scan | tee "$evidence/orphan.json"
grep -q '"needs_review": false' "$evidence/orphan.json"
/usr/local/bin/vpskit doctor | tee "$evidence/doctor.json"
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-certificate-renew.timer

rm -f -- "$release_archive" "$crash_archive"
rm -rf -- "$bundle" "$crash_bundle"
printf 'LAB23_CRASH_RECOVERY=PASS\n'
printf 'KILL9_INCOMPLETE_DETECTION=PASS\n'
printf 'AUTOMATIC_CORE_RECOVERY=PASS\n'
printf 'RECOVERED_HASHES_MATCH=PASS\n'
