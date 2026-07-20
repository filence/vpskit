#!/usr/bin/env bash
set -eu

base="$(mktemp -d /var/tmp/vpskit-permission-verify.XXXXXX)"
cleanup() {
    resolved="$(readlink -f -- "$base" 2>/dev/null || true)"
    case "$resolved" in
        /var/tmp/vpskit-permission-verify.*) rm -rf -- "$resolved" ;;
        *) echo 'cleanup_refused=unexpected_path' >&2 ;;
    esac
}
trap cleanup EXIT HUP INT TERM

chown root:nogroup "$base"
chmod 0750 "$base"
install -d -o root -g nogroup -m 0750 "$base/certificates"
printf '%s\n' 'fixture' > "$base/certificates/hysteria2.crt"
chown root:nogroup "$base/certificates/hysteria2.crt"
chmod 0640 "$base/certificates/hysteria2.crt"

runuser -u nobody -- test -x "$base"
runuser -u nobody -- test -r "$base/certificates/hysteria2.crt"
echo 'PERMISSION_MODEL=GREEN'
