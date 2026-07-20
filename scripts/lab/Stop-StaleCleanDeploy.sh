#!/usr/bin/env bash
set -Eeuo pipefail

for pid in 40779 40743; do
    if kill -0 "$pid" 2>/dev/null; then
        kill -TERM "$pid" 2>/dev/null || true
    fi
done
for _ in $(seq 1 30); do
    alive=0
    for pid in 40779 40743; do
        if kill -0 "$pid" 2>/dev/null; then alive=1; fi
    done
    if [ "$alive" -eq 0 ]; then break; fi
    sleep 1
done
for pid in 40779 40743; do
    if kill -0 "$pid" 2>/dev/null; then
        kill -KILL "$pid" 2>/dev/null || true
    fi
done
rm -rf -- /root/vpskit-lab-v0.1.0-lab.16-clean /root/vpskit-lab-v0.1.0-lab.16-debug
rm -f -- /root/v0.1.0-lab.16-linux-amd64.tar.gz /root/.vpskit-cf-token
systemctl reset-failed vpskit-sing-box.service >/dev/null 2>&1 || true
echo 'STALE_CLEAN_DEPLOY_STOP=PASS'
