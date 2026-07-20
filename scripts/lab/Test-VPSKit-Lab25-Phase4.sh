#!/usr/bin/env bash
set -Eeuo pipefail

vpskit='/usr/local/bin/vpskit'
sing_box='/usr/local/lib/vpskit/bin/sing-box'
test_root="$(mktemp -d /var/tmp/vpskit-lab25-phase4.XXXXXX)"
client_pid=''

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
    rm -rf -- /root/vpskit-lab25-systemctl-shim
    resolved="$(readlink -f -- "$test_root" 2>/dev/null || true)"
    case "$resolved" in
        /var/tmp/vpskit-lab25-phase4.*) rm -rf -- "$resolved" ;;
        *) printf 'cleanup refused for unexpected test root\n' >&2 ;;
    esac
}
trap cleanup EXIT HUP INT TERM

transaction_id() {
    python3 -c 'import json,sys; print(json.loads(sys.argv[1])["detail"]["transaction_id"])' "$1"
}

assert_state() {
    expected_profile="$1"
    expected_reality="$2"
    expected_hysteria2="$3"
    expected_tcp="$4"
    expected_udp="$5"
    python3 - "$expected_profile" "$expected_reality" "$expected_hysteria2" "$expected_tcp" "$expected_udp" <<'PY'
import json
import sys
from pathlib import Path

profile, reality, hysteria2, tcp_port, udp_port = sys.argv[1:]
state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
assert state['schema_version'] == 3
assert state['profile'] == profile, (state['profile'], profile)
assert state['reality'].get('enabled', False) is (reality == 'true')
assert state['hysteria2'].get('enabled', False) is (hysteria2 == 'true')
if state['reality'].get('id'):
    assert state['reality']['listen_port'] == int(tcp_port)
if state['hysteria2'].get('id'):
    assert state['hysteria2']['listen_port'] == int(udp_port)

server = json.loads(Path('/etc/vpskit/generated/sing-box.json').read_text(encoding='utf-8'))
inbound_types = {item['type'] for item in server['inbounds']}
assert ('vless' in inbound_types) is (reality == 'true')
assert ('hysteria2' in inbound_types) is (hysteria2 == 'true')

reality_export = Path('/etc/vpskit/exports/sing-box-reality.json')
hysteria2_export = Path('/etc/vpskit/exports/sing-box-hysteria2.json')
assert reality_export.exists() is (reality == 'true')
assert hysteria2_export.exists() is (hysteria2 == 'true')

mihomo = Path('/etc/vpskit/exports/mihomo.yaml').read_text(encoding='utf-8')
links = Path('/etc/vpskit/exports/share-links.txt').read_text(encoding='utf-8')
assert ('JP-Reality' in mihomo) is (reality == 'true')
assert ('JP-Hysteria2' in mihomo) is (hysteria2 == 'true')
assert ('vless://' in links) is (reality == 'true')
assert ('hysteria2://' in links) is (hysteria2 == 'true')
PY
    "$sing_box" check -c /etc/vpskit/generated/sing-box.json
    "$vpskit" doctor >"$test_root/doctor.json"
    python3 - "$test_root/doctor.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding='utf-8') as handle:
    result = json.load(handle)
assert result['status'] == 'PASS', result
PY
}

run_client() {
    protocol="$1"
    source_path="$2"
    outbound_type="$3"
    socks_port="$4"
    config_path="$test_root/$protocol.json"
    log_path="$test_root/$protocol.log"
    python3 - "$source_path" "$config_path" "$outbound_type" "$socks_port" <<'PY'
import json
import sys
from pathlib import Path

source, destination, outbound_type, socks_port = sys.argv[1:]
configuration = json.loads(Path(source).read_text(encoding='utf-8'))
configuration['log']['level'] = 'error'
configuration['inbounds'][0]['listen_port'] = int(socks_port)
outbound = next(item for item in configuration['outbounds'] if item['type'] == outbound_type)
outbound['server'] = '127.0.0.1'
Path(destination).write_text(json.dumps(configuration, indent=2) + '\n', encoding='utf-8')
PY
    chmod 0600 "$config_path"
    "$sing_box" check -c "$config_path"
    "$sing_box" run -c "$config_path" >"$log_path" 2>&1 &
    client_pid=$!
    for _ in $(seq 1 50); do
        if ss -ltnH | grep -q ":$socks_port "; then
            break
        fi
        kill -0 "$client_pid" 2>/dev/null
        sleep 0.1
    done
    ss -ltnH | grep -q ":$socks_port "
    curl --silent --show-error --fail --max-time 20 --socks5-hostname "127.0.0.1:$socks_port" https://api.ipify.org >"$test_root/$protocol.ip"
    grep -Eq '^[0-9a-fA-F:.]+$' "$test_root/$protocol.ip"
    cleanup_client
    printf 'LOOPBACK_%s=PASS\n' "$protocol"
}

test_active_clients() {
    reality_enabled="$(python3 -c 'import json; print(str(json.load(open("/var/lib/vpskit/state.json"))["reality"].get("enabled", False)).lower())')"
    hysteria2_enabled="$(python3 -c 'import json; print(str(json.load(open("/var/lib/vpskit/state.json"))["hysteria2"].get("enabled", False)).lower())')"
    if [ "$reality_enabled" = true ]; then
        run_client reality /etc/vpskit/exports/sing-box-reality.json vless 12080
    fi
    if [ "$hysteria2_enabled" = true ]; then
        run_client hysteria2 /etc/vpskit/exports/sing-box-hysteria2.json hysteria2 12081
    fi
}

assert_state balanced true true 443 443
test_active_clients

output="$($vpskit instance disable hysteria2)"
printf '%s\n' "$output"
assert_state reality-only true false 443 443
test_active_clients

output="$($vpskit instance enable hysteria2)"
printf '%s\n' "$output"
assert_state balanced true true 443 443
test_active_clients

output="$($vpskit instance disable reality)"
printf '%s\n' "$output"
assert_state hysteria2-only false true 443 443
test_active_clients

output="$($vpskit instance enable reality)"
printf '%s\n' "$output"
assert_state balanced true true 443 443

output="$($vpskit instance modify hysteria2 --port 24443)"
printf '%s\n' "$output"
modify_hysteria2_tx="$(transaction_id "$output")"
assert_state balanced true true 443 24443
test_active_clients
$vpskit rollback "$modify_hysteria2_tx" --yes
assert_state balanced true true 443 443

output="$($vpskit instance delete hysteria2 --yes)"
printf '%s\n' "$output"
delete_hysteria2_tx="$(transaction_id "$output")"
assert_state reality-only true false 443 0
test_active_clients
! systemctl is-active --quiet vpskit-certificate-renew.timer
$vpskit rollback "$delete_hysteria2_tx" --yes
assert_state balanced true true 443 443
systemctl is-active --quiet vpskit-certificate-renew.timer

output="$($vpskit instance delete reality --yes)"
printf '%s\n' "$output"
delete_reality_tx="$(transaction_id "$output")"
assert_state hysteria2-only false true 0 443
test_active_clients
$vpskit rollback "$delete_reality_tx" --yes
assert_state balanced true true 443 443

install -d -m 0700 /root/vpskit-lab25-systemctl-shim
real_systemctl="$(command -v systemctl)"
cat > /root/vpskit-lab25-systemctl-shim/systemctl <<EOF
#!/usr/bin/env bash
set -eu
"$real_systemctl" "\$@"
if [ "\${1:-}" = restart ] && [ "\${2:-}" = vpskit-sing-box.service ]; then
    kill -KILL "\$PPID"
fi
EOF
chmod 0700 /root/vpskit-lab25-systemctl-shim/systemctl
set +e
PATH="/root/vpskit-lab25-systemctl-shim:$PATH" "$vpskit" instance modify reality --port 24443 >"$test_root/crash-output.json" 2>"$test_root/crash-error.log"
crash_code=$?
set -e
test "$crash_code" -ne 0
rm -rf -- /root/vpskit-lab25-systemctl-shim

python3 - <<'PY'
import json
from pathlib import Path

records = []
for path in Path('/var/lib/vpskit/transactions').glob('TX-*/transaction.json'):
    try:
        record = json.loads(path.read_text(encoding='utf-8'))
    except Exception:
        continue
    if record.get('status') == 'IN_PROGRESS' and record.get('command') == 'instance modify':
        records.append((path.stat().st_mtime_ns, path, record))
assert records, 'missing interrupted instance transaction'
_, _, record = max(records)
assert record.get('previous_backup_id'), record
state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
assert state['reality']['listen_port'] == 24443, state['reality']['listen_port']
print('INSTANCE_CRASH_IN_PROGRESS=PASS')
PY

recovery_output="$($vpskit recover)"
printf '%s\n' "$recovery_output"
grep -Fq 'RECOVERED' <<<"$recovery_output"
assert_state balanced true true 443 443
test_active_clients

$vpskit export --format qr >"$test_root/qr.txt"
grep -Fq 'VPSKit REALITY QR' "$test_root/qr.txt"
grep -Fq 'VPSKit HYSTERIA2 QR' "$test_root/qr.txt"
! grep -Eq 'vless://|hysteria2://' "$test_root/qr.txt"
printf 'QR_EXPORT=PASS\n'

status_output="$($vpskit status)"
python3 - "$status_output" <<'PY'
import json
import sys

result = json.loads(sys.argv[1])
assert result['status'] == 'PASS', result
detail = result['detail']
assert detail['incomplete_transactions'] == [], detail['incomplete_transactions']
assert detail['incomplete_core_transactions'] == [], detail['incomplete_core_transactions']
assert detail['profile'] == 'balanced'
assert detail['reality_enabled'] is True
assert detail['hysteria2_enabled'] is True
PY

orphan_output="$($vpskit orphan scan)"
python3 - "$orphan_output" <<'PY'
import json
import sys

result = json.loads(sys.argv[1])
assert result['status'] == 'PASS', result
assert (result.get('detail') or {}).get('needs_review') in (False, None), result
PY

printf 'LAB25_PHASE4_LIFECYCLE=PASS\n'
