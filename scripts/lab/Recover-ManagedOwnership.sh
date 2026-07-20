#!/usr/bin/env bash
set -Eeuo pipefail

chown root:vpskit /etc/vpskit /etc/vpskit/generated /etc/vpskit/generated/sing-box.json
chown root:vpskit /var/lib/vpskit /var/lib/vpskit/certificates
chown root:vpskit /var/lib/vpskit/certificates/hysteria2.crt /var/lib/vpskit/certificates/hysteria2.key
chmod 0750 /etc/vpskit /etc/vpskit/generated /var/lib/vpskit /var/lib/vpskit/certificates
chmod 0640 /etc/vpskit/generated/sing-box.json /var/lib/vpskit/certificates/hysteria2.crt /var/lib/vpskit/certificates/hysteria2.key
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
stat -c '%a %U:%G %n' /etc/vpskit/generated/sing-box.json /var/lib/vpskit/certificates/hysteria2.crt /var/lib/vpskit/certificates/hysteria2.key
echo 'MANAGED_OWNERSHIP_RECOVERY=PASS'
