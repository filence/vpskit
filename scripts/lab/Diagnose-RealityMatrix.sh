#!/usr/bin/env bash
set -euo pipefail

sing_box='/usr/local/lib/vpskit/bin/sing-box'
matrix_root="$(mktemp -d /run/vpskit-reality-matrix.XXXXXX)"
server_pid=''
client_pid=''
cleanup_processes() {
    if [ -n "$client_pid" ]; then kill "$client_pid" 2>/dev/null || true; fi
    if [ -n "$server_pid" ]; then kill "$server_pid" 2>/dev/null || true; fi
    client_pid=''
    server_pid=''
}
cleanup() {
    cleanup_processes
    rm -rf -- "$matrix_root"
}
trap cleanup EXIT HUP INT TERM

python3 - "$matrix_root" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
state = json.loads(pathlib.Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
secrets = json.loads(pathlib.Path('/var/lib/vpskit/secrets/instances.json').read_text(encoding='utf-8'))

fixture_private = 'UuMBgl7MXTPx9inmQp2UC7Jcnwc6XYbwDNebonM-FCc'
fixture_public = 'jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0'
fixture_short = '0123456789abcdef'

cases = [
    ('fixture_microsoft', fixture_private, fixture_public, fixture_short, 'www.microsoft.com'),
    ('managed_google', secrets['reality_private_key'], state['reality']['public_key'], state['reality']['short_id'], 'google.com'),
    ('managed_fixed_short_google', secrets['reality_private_key'], state['reality']['public_key'], fixture_short, 'google.com'),
    ('managed_amazon', secrets['reality_private_key'], state['reality']['public_key'], state['reality']['short_id'], 'www.amazon.com'),
]

for index, (name, private_key, public_key, short_id, target) in enumerate(cases):
    server_port = 14501 + index
    client_port = 17781 + index
    server = {
        'log': {'level': 'trace'},
        'inbounds': [{
            'type': 'vless', 'tag': 'matrix-server', 'listen': '127.0.0.1',
            'listen_port': server_port,
            'users': [{'uuid': secrets['reality_uuid'], 'flow': 'xtls-rprx-vision'}],
            'tls': {
                'enabled': True, 'server_name': target,
                'reality': {
                    'enabled': True,
                    'handshake': {'server': target, 'server_port': 443},
                    'private_key': private_key,
                    'short_id': [short_id],
                    'max_time_difference': '1m',
                },
            },
        }],
        'outbounds': [{'type': 'direct', 'tag': 'direct'}],
        'route': {'final': 'direct'},
    }
    client = {
        'log': {'level': 'trace'},
        'inbounds': [{'type': 'mixed', 'listen': '127.0.0.1', 'listen_port': client_port}],
        'outbounds': [{
            'type': 'vless', 'tag': 'matrix-client', 'server': '127.0.0.1',
            'server_port': server_port, 'uuid': secrets['reality_uuid'],
            'flow': 'xtls-rprx-vision', 'network': 'tcp',
            'tls': {
                'enabled': True, 'server_name': target,
                'utls': {'enabled': True, 'fingerprint': 'chrome'},
                'reality': {'enabled': True, 'public_key': public_key, 'short_id': short_id},
            },
        }, {'type': 'direct', 'tag': 'direct'}],
        'route': {'final': 'matrix-client'},
    }
    case_root = root / name
    case_root.mkdir(mode=0o700)
    for filename, content in [('server.json', server), ('client.json', client)]:
        path = case_root / filename
        path.write_text(json.dumps(content), encoding='utf-8')
        path.chmod(0o600)
    (case_root / 'ports').write_text(f'{server_port} {client_port}\n', encoding='ascii')
PY

run_case() {
    local case_name="$1"
    local case_root="$matrix_root/$case_name"
    local server_port client_port
    read -r server_port client_port < "$case_root/ports"

    "$sing_box" check -c "$case_root/server.json" >/dev/null
    "$sing_box" check -c "$case_root/client.json" >/dev/null
    "$sing_box" run -c "$case_root/server.json" >"$case_root/server.log" 2>&1 &
    server_pid=$!
    for _ in $(seq 1 40); do
        ss -ltnH | grep -q "127.0.0.1:$server_port " && break
        kill -0 "$server_pid" 2>/dev/null || break
        sleep 0.1
    done
    ss -ltnH | grep -q "127.0.0.1:$server_port "

    "$sing_box" run -c "$case_root/client.json" >"$case_root/client.log" 2>&1 &
    client_pid=$!
    for _ in $(seq 1 40); do
        ss -ltnH | grep -q "127.0.0.1:$client_port " && break
        kill -0 "$client_pid" 2>/dev/null || break
        sleep 0.1
    done
    ss -ltnH | grep -q "127.0.0.1:$client_port "

    if curl --silent --show-error --fail --max-time 15 --socks5-hostname "127.0.0.1:$client_port" https://api.ipify.org >/dev/null 2>&1; then
        echo "REALITY_MATRIX=$case_name result=PASS"
    else
        local client_failed=false
        local server_failed=false
        grep -Eq 'reality verification failed|REALITY: received real certificate' "$case_root/client.log" && client_failed=true
        grep -q 'REALITY: processed invalid connection' "$case_root/server.log" && server_failed=true
        echo "REALITY_MATRIX=$case_name result=FAIL client_auth_failure=$client_failed server_invalid_connection=$server_failed"
    fi
    cleanup_processes
}

run_case fixture_microsoft
run_case managed_google
run_case managed_fixed_short_google
run_case managed_amazon
