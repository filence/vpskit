#!/usr/bin/env bash
set -Eeuo pipefail

certificate='/var/lib/vpskit/certificates/hysteria2.crt'
key='/var/lib/vpskit/certificates/hysteria2.key'
for path in "$certificate" "$key"; do
    case "$path" in
        /var/lib/vpskit/certificates/hysteria2.crt|/var/lib/vpskit/certificates/hysteria2.key) ;;
        *) echo 'RECOVERY_REFUSED=unexpected_path' >&2; exit 1 ;;
    esac
    test -f "$path"
done
chown root:vpskit "$certificate" "$key"
chmod 0640 "$certificate" "$key"
systemctl reset-failed vpskit-sing-box.service || true
systemctl restart vpskit-sing-box.service
for _ in $(seq 1 30); do
    if systemctl is-active --quiet vpskit-sing-box.service && ss -ltnH | grep -q ':443 ' && ss -lunH | grep -q ':443 '; then
        break
    fi
    sleep 0.5
done
systemctl is-active --quiet vpskit-sing-box.service
ss -ltnH | grep -q ':443 '
ss -lunH | grep -q ':443 '
stat -c '%a %U:%G %n' "$certificate" "$key"
echo 'CERTIFICATE_PERMISSION_RECOVERY=PASS'
