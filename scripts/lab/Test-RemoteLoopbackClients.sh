#!/usr/bin/env bash
set -eu

binary='/usr/local/lib/vpskit/bin/sing-box'
base="$(mktemp -d /var/tmp/vpskit-loopback-client.XXXXXX)"
client_pid=''

cleanup_client() {
    if [ -n "$client_pid" ] && kill -0 "$client_pid" 2>/dev/null; then
        kill "$client_pid" 2>/dev/null || true
        wait "$client_pid" 2>/dev/null || true
    fi
    client_pid=''
}

cleanup() {
    cleanup_client
    resolved="$(readlink -f -- "$base" 2>/dev/null || true)"
    case "$resolved" in
        /var/tmp/vpskit-loopback-client.*) rm -rf -- "$resolved" ;;
        *) echo 'cleanup_refused=unexpected_path' >&2 ;;
    esac
}
trap cleanup EXIT HUP INT TERM

make_config() {
    source_path="$1"
    destination_path="$2"
    outbound_type="$3"
    socks_port="$4"
    python3 - "$source_path" "$destination_path" "$outbound_type" "$socks_port" <<'PY'
import json
import sys
from pathlib import Path

source, destination, outbound_type, socks_port = sys.argv[1:]
configuration = json.loads(Path(source).read_text())
configuration['log']['level'] = 'debug'
configuration['inbounds'][0]['listen_port'] = int(socks_port)
outbound = next(item for item in configuration['outbounds'] if item['type'] == outbound_type)
outbound['server'] = '127.0.0.1'
Path(destination).write_text(json.dumps(configuration, indent=2) + '\n')
PY
    chmod 0600 "$destination_path"
    "$binary" check -c "$destination_path"
}

wait_for_port() {
    port="$1"
    attempts=0
    while [ "$attempts" -lt 40 ]; do
        if ! kill -0 "$client_pid" 2>/dev/null; then
            return 1
        fi
        if ss -ltnH | grep -q ":$port "; then
            return 0
        fi
        attempts=$((attempts + 1))
        sleep 0.1
    done
    return 1
}

test_profile() {
    name="$1"
    source_path="$2"
    outbound_type="$3"
    socks_port="$4"
    config_path="$base/$name.json"
    log_path="$base/$name.log"
    make_config "$source_path" "$config_path" "$outbound_type" "$socks_port"
    "$binary" run -c "$config_path" > "$log_path" 2>&1 &
    client_pid=$!
    if ! wait_for_port "$socks_port"; then
        grep 'ERROR' "$log_path" || true
        echo "REMOTE_LOOPBACK=FAIL profile=$name reason=client_not_ready"
        exit 1
    fi
    set +e
    response="$(curl --silent --show-error --fail --max-time 20 --socks5-hostname "127.0.0.1:$socks_port" https://api.ipify.org 2> "$base/$name.curl-error")"
    curl_code=$?
    set -e
    if [ "$curl_code" -ne 0 ] || ! printf '%s' "$response" | grep -Eq '^[0-9a-fA-F:.]+$'; then
        sleep 1
        grep 'ERROR' "$log_path" || true
        echo "REMOTE_LOOPBACK=FAIL profile=$name curl_code=$curl_code"
        exit 1
    fi
    cleanup_client
    echo "REMOTE_LOOPBACK=PASS profile=$name"
}

test_profile reality /etc/vpskit/exports/sing-box-reality.json vless 12080
test_profile hysteria2 /etc/vpskit/exports/sing-box-hysteria2.json hysteria2 12081
echo 'REMOTE_LOOPBACK_CLIENTS=PASS'
