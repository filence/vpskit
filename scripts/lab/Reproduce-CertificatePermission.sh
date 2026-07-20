#!/usr/bin/env bash
set -eu

base="$(mktemp -d /var/tmp/vpskit-permission-repro.XXXXXX)"
cleanup() {
    resolved="$(readlink -f -- "$base" 2>/dev/null || true)"
    case "$resolved" in
        /var/tmp/vpskit-permission-repro.*) rm -rf -- "$resolved" ;;
        *) echo 'cleanup_refused=unexpected_path' >&2 ;;
    esac
}
trap cleanup EXIT HUP INT TERM

chown root:root "$base"
chmod 0750 "$base"
install -d -o root -g nogroup -m 0750 "$base/certificates"
printf '%s\n' 'fixture' > "$base/certificates/hysteria2.crt"
chown root:nogroup "$base/certificates/hysteria2.crt"
chmod 0640 "$base/certificates/hysteria2.crt"

if runuser -u nobody -- test -r "$base/certificates/hysteria2.crt"; then
    echo 'REPRO=UNEXPECTED_GREEN'
    exit 2
fi

echo 'REPRO=RED reason=parent_directory_not_traversable'
exit 1
