#!/usr/bin/env bash
set -euo pipefail

sing_box='/usr/local/lib/vpskit/bin/sing-box'
fixture_root="$(mktemp -d /run/vpskit-reality-fixture.XXXXXX)"
server_pid=''
client_pid=''
cleanup() {
    if [ -n "$client_pid" ]; then kill "$client_pid" 2>/dev/null || true; fi
    if [ -n "$server_pid" ]; then kill "$server_pid" 2>/dev/null || true; fi
    rm -rf -- "$fixture_root"
}
trap cleanup EXIT HUP INT TERM

key_output="$($sing_box generate reality-keypair)"
private_key="$(printf '%s\n' "$key_output" | sed -n 's/^PrivateKey:[[:space:]]*//p')"
public_key="$(printf '%s\n' "$key_output" | sed -n 's/^PublicKey:[[:space:]]*//p')"
test -n "$private_key"
test -n "$public_key"

cat > "$fixture_root/server.json" <<'JSON'
{
  "log": {"level": "trace"},
  "inbounds": [{
    "type": "vless",
    "tag": "fixture-server",
    "listen": "127.0.0.1",
    "listen_port": 14443,
    "users": [{"uuid": "d342d11e-d424-4583-b36e-524ab1f0afa4", "flow": "xtls-rprx-vision"}],
    "tls": {
      "enabled": true,
      "server_name": "google.com",
      "reality": {
        "enabled": true,
        "handshake": {"server": "google.com", "server_port": 443},
        "private_key": "__REALITY_PRIVATE_KEY__",
        "short_id": ["0123456789abcdef"],
        "max_time_difference": "1m"
      }
    }
  }],
  "outbounds": [{"type": "direct", "tag": "direct"}],
  "route": {"final": "direct"}
}
JSON

cat > "$fixture_root/client.json" <<'JSON'
{
  "log": {"level": "trace"},
  "inbounds": [{"type": "mixed", "listen": "127.0.0.1", "listen_port": 17777}],
  "outbounds": [{
    "type": "vless",
    "tag": "fixture-client",
    "server": "127.0.0.1",
    "server_port": 14443,
    "uuid": "d342d11e-d424-4583-b36e-524ab1f0afa4",
    "flow": "xtls-rprx-vision",
    "network": "tcp",
    "tls": {
      "enabled": true,
      "server_name": "google.com",
      "utls": {"enabled": true, "fingerprint": "chrome"},
      "reality": {
        "enabled": true,
        "public_key": "__REALITY_PUBLIC_KEY__",
        "short_id": "0123456789abcdef"
      }
    }
  }, {"type": "direct", "tag": "direct"}],
  "route": {"final": "fixture-client"}
}
JSON

sed -i "s|__REALITY_PRIVATE_KEY__|$private_key|" "$fixture_root/server.json"
sed -i "s|__REALITY_PUBLIC_KEY__|$public_key|" "$fixture_root/client.json"

chmod 0600 "$fixture_root/server.json" "$fixture_root/client.json"
"$sing_box" check -c "$fixture_root/server.json"
"$sing_box" check -c "$fixture_root/client.json"

"$sing_box" run -c "$fixture_root/server.json" >"$fixture_root/server.log" 2>&1 &
server_pid=$!
for _ in $(seq 1 40); do
    ss -ltnH | grep -q '127.0.0.1:14443 ' && break
    kill -0 "$server_pid" 2>/dev/null || break
    sleep 0.1
done
ss -ltnH | grep -q '127.0.0.1:14443 '

"$sing_box" run -c "$fixture_root/client.json" >"$fixture_root/client.log" 2>&1 &
client_pid=$!
for _ in $(seq 1 40); do
    ss -ltnH | grep -q '127.0.0.1:17777 ' && break
    kill -0 "$client_pid" 2>/dev/null || break
    sleep 0.1
done
ss -ltnH | grep -q '127.0.0.1:17777 '

if curl --silent --show-error --fail --max-time 15 --socks5-hostname 127.0.0.1:17777 https://api.ipify.org >/dev/null 2>&1; then
    echo 'UPSTREAM_REALITY_FIXTURE=PASS'
    exit 0
fi

client_failed=false
server_failed=false
if grep -Eq 'reality verification failed|REALITY: received real certificate' "$fixture_root/client.log"; then
    client_failed=true
fi
if grep -q 'REALITY: processed invalid connection' "$fixture_root/server.log"; then
    server_failed=true
fi
echo "fixture_client_auth_failure=$client_failed"
echo "fixture_server_invalid_connection=$server_failed"
if [ "$client_failed" = true ] && [ "$server_failed" = true ]; then
    echo 'UPSTREAM_REALITY_FIXTURE=REPRODUCED'
    exit 0
fi
echo 'UPSTREAM_REALITY_FIXTURE=INCONCLUSIVE'
exit 1
