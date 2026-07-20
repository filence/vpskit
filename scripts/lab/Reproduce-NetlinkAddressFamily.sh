#!/usr/bin/env bash
set -eu

source_binary='/root/vpskit-lab-v0.1.0-lab.3/v0.1.0-lab.3/sing-box'
base="$(mktemp -d /var/tmp/vpskit-netlink-repro.XXXXXX)"
chmod 0755 "$base"
cleanup() {
    resolved="$(readlink -f -- "$base" 2>/dev/null || true)"
    case "$resolved" in
        /var/tmp/vpskit-netlink-repro.*) rm -rf -- "$resolved" ;;
        *) echo 'cleanup_refused=unexpected_path' >&2 ;;
    esac
}
trap cleanup EXIT HUP INT TERM

install -m 0755 "$source_binary" "$base/sing-box"
cat > "$base/config.json" <<'JSON'
{
  "log": { "level": "info", "timestamp": false },
  "inbounds": [],
  "outbounds": [{ "type": "direct", "tag": "direct" }],
  "route": { "final": "direct" }
}
JSON
chmod 0644 "$base/config.json"
"$base/sing-box" check -c "$base/config.json"

set +e
denied_output="$(systemd-run --quiet --wait --pipe --collect \
    --unit="vpskit-netlink-denied-$$" \
    -p User=nobody \
    -p Group=nogroup \
    -p 'RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6' \
    /usr/bin/timeout 3s "$base/sing-box" run -c "$base/config.json" 2>&1)"
denied_code=$?
allowed_output="$(systemd-run --quiet --wait --pipe --collect \
    --unit="vpskit-netlink-allowed-$$" \
    -p User=nobody \
    -p Group=nogroup \
    -p 'RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK' \
    /usr/bin/timeout 3s "$base/sing-box" run -c "$base/config.json" 2>&1)"
allowed_code=$?
set -e

if ! printf '%s' "$denied_output" | grep -q 'subscribe route updates: address family not supported by protocol'; then
    echo "NETLINK_AB=FAIL denied_code=$denied_code reason=original_error_not_reproduced"
    printf '%s\n' "$denied_output"
    exit 1
fi
if printf '%s' "$allowed_output" | grep -q 'subscribe route updates'; then
    echo "NETLINK_AB=FAIL allowed_code=$allowed_code reason=error_persisted_with_af_netlink"
    printf '%s\n' "$allowed_output"
    exit 1
fi
if ! printf '%s' "$allowed_output" | grep -q 'sing-box started'; then
    echo "NETLINK_AB=FAIL allowed_code=$allowed_code reason=process_did_not_start"
    printf '%s\n' "$allowed_output"
    exit 1
fi

echo "NETLINK_AB=PASS denied_code=$denied_code allowed_code=$allowed_code only_change=AF_NETLINK"
