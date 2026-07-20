#!/usr/bin/env bash
set -Eeuo pipefail

pkill -TERM -f '/root/vpskit-lab-v0.1.0-lab.8-cert/v0.1.0-lab.8/vpskit cert renew --force' || true
pkill -TERM -f '/usr/local/lib/vpskit/bin/lego run .* /var/lib/vpskit/transactions/TX-.*-cert-renew-.*/staging/lego' || true
sleep 2
pkill -KILL -f '/root/vpskit-lab-v0.1.0-lab.8-cert/v0.1.0-lab.8/vpskit cert renew --force' || true
pkill -KILL -f '/usr/local/lib/vpskit/bin/lego run .* /var/lib/vpskit/transactions/TX-.*-cert-renew-.*/staging/lego' || true
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
find /var/lib/vpskit/transactions -mindepth 2 -maxdepth 2 -type d -name staging -print
echo 'LAB8_RENEWAL_ABORT=PASS'
