#!/usr/bin/env bash
set -Eeuo pipefail

vpskit=/usr/local/bin/vpskit
sing_box=/usr/local/lib/vpskit/bin/sing-box
xray=/usr/local/lib/vpskit/bin/xray
test_root=/root/vpskit-lab31-lifecycle
client_pid=''
expected_ip=''

cleanup_client() {
  if [ -n "$client_pid" ] && kill -0 "$client_pid" 2>/dev/null; then
    kill "$client_pid" 2>/dev/null || true
    wait "$client_pid" 2>/dev/null || true
  fi
  client_pid=''
}

cleanup() {
  set +e
  cleanup_client
  rm -rf -- "$test_root"
}
trap cleanup EXIT HUP INT TERM

rm -rf -- "$test_root"
install -d -m 0700 "$test_root"

hash_file() {
  sha256sum "$1" | awk '{print $1}'
}

assert_profile() {
  expected_profile="$1"
  expected_reality="$2"
  expected_hysteria2="$3"
  python3 - "$expected_profile" "$expected_reality" "$expected_hysteria2" <<'PY'
import json
import sys

profile, reality, hysteria2 = sys.argv[1:]
with open('/var/lib/vpskit/state.json', encoding='utf-8') as stream:
    state = json.load(stream)
assert state['schema_version'] == 4, state
assert state['profile'] == profile, state
assert state['reality']['enabled'] is (reality == 'true'), state
assert state['hysteria2']['enabled'] is (hysteria2 == 'true'), state
assert state['core']['id'] == 'sing-box', state
assert state['reality_core']['id'] == 'xray', state
PY
}

wait_socks() {
  port="$1"
  for _ in $(seq 1 100); do
    if ! kill -0 "$client_pid" 2>/dev/null; then
      echo "client exited before SOCKS port $port became ready" >&2
      return 1
    fi
    if python3 - "$port" <<'PY'
import socket
import sys

sock = socket.socket()
sock.settimeout(0.1)
try:
    sock.connect(('127.0.0.1', int(sys.argv[1])))
except OSError:
    raise SystemExit(1)
finally:
    sock.close()
PY
    then
      return 0
    fi
    sleep 0.1
  done
  return 1
}

run_client() {
  name="$1"
  config="$2"
  port="$3"
  cleanup_client
  "$sing_box" check -c "$config"
  "$sing_box" run -c "$config" >"$test_root/$name.stdout" 2>"$test_root/$name.stderr" &
  client_pid=$!
  wait_socks "$port"
  actual_ip="$(curl --silent --show-error --fail --max-time 30 --socks5-hostname "127.0.0.1:$port" https://api.ipify.org)"
  test "$actual_ip" = "$expected_ip"
  cleanup_client
  printf 'LOOPBACK_CLIENT=PASS profile=%s\n' "$name"
}

assert_balanced_services() {
  systemctl is-active --quiet vpskit-xray.service
  systemctl is-active --quiet vpskit-sing-box.service
  systemctl is-active --quiet vpskit-certificate-renew.timer
}

test "$($vpskit version)" = 'vpskit v0.1.0-lab.31'
expected_ip="$(curl --silent --show-error --fail --max-time 30 https://api.ipify.org)"
assert_profile balanced true true
assert_balanced_services

baseline_xray="$(hash_file /etc/vpskit/generated/xray.json)"
baseline_sing_box="$(hash_file /etc/vpskit/generated/sing-box.json)"
baseline_certificate="$(hash_file /var/lib/vpskit/certificates/hysteria2.crt)"
baseline_key="$(hash_file /var/lib/vpskit/certificates/hysteria2.key)"
baseline_mihomo="$(hash_file /etc/vpskit/exports/mihomo.yaml)"

backup_json="$($vpskit backup)"
backup_id="$(python3 -c 'import json,sys; value=json.loads(sys.argv[1]); assert value["status"] == "PASS", value; print(value["detail"]["backup_id"])' "$backup_json")"
case "$backup_id" in BK-*) ;; *) echo 'invalid backup id' >&2; exit 1 ;; esac
test -f "/var/lib/vpskit/backups/$backup_id/backup.json"
echo 'LAB31_BACKUP_CREATE=PASS'

set +e
$vpskit restore "$backup_id" >"$test_root/restore-no-confirm.stdout" 2>"$test_root/restore-no-confirm.stderr"
restore_gate_exit=$?
set -e
test "$restore_gate_exit" -ne 0
grep -F -- '--yes' "$test_root/restore-no-confirm.stderr" >/dev/null
echo 'LAB31_RESTORE_CONFIRMATION_GATE=PASS'

$vpskit instance disable hysteria2 >/dev/null
assert_profile reality-only true false
systemctl is-active --quiet vpskit-xray.service
! systemctl is-active --quiet vpskit-sing-box.service
run_client reality /etc/vpskit/exports/sing-box-reality.json 2080
echo 'LAB31_DISABLE_HYSTERIA2=PASS'

$vpskit instance enable hysteria2 >/dev/null
assert_profile balanced true true
assert_balanced_services
run_client hysteria2 /etc/vpskit/exports/sing-box-hysteria2.json 2081
echo 'LAB31_ENABLE_HYSTERIA2=PASS'

$vpskit instance disable reality >/dev/null
assert_profile hysteria2-only false true
! systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
run_client hysteria2 /etc/vpskit/exports/sing-box-hysteria2.json 2081
echo 'LAB31_DISABLE_REALITY=PASS'

$vpskit restore "$backup_id" --yes >/dev/null
test "$($vpskit version)" = 'vpskit v0.1.0-lab.31'
assert_profile balanced true true
assert_balanced_services
test "$(hash_file /etc/vpskit/generated/xray.json)" = "$baseline_xray"
test "$(hash_file /etc/vpskit/generated/sing-box.json)" = "$baseline_sing_box"
test "$(hash_file /var/lib/vpskit/certificates/hysteria2.crt)" = "$baseline_certificate"
test "$(hash_file /var/lib/vpskit/certificates/hysteria2.key)" = "$baseline_key"
test "$(hash_file /etc/vpskit/exports/mihomo.yaml)" = "$baseline_mihomo"
"$xray" run -test -config /etc/vpskit/generated/xray.json >/dev/null
"$sing_box" check -c /etc/vpskit/generated/sing-box.json
$vpskit doctor >/dev/null
run_client reality /etc/vpskit/exports/sing-box-reality.json 2080
run_client hysteria2 /etc/vpskit/exports/sing-box-hysteria2.json 2081
echo 'LAB31_BACKUP_RESTORE=PASS'
echo "LAB31_LIFECYCLE=PASS backup_id=$backup_id"
