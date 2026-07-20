#!/usr/bin/env bash
set -Eeuo pipefail

targets=(
    '/root/v0.1.0-lab.19-linux-amd64.tar.gz'
    '/root/vpskit-lab-v0.1.0-lab.19-zerossl-clean'
    '/root/.vpskit-cf-token'
    '/root/.vpskit-acme-email'
)
for target in "${targets[@]}"; do
    if [ -L "$target" ]; then
        echo "CLEANUP_REFUSED symlink=$target" >&2
        exit 1
    fi
    rm -rf -- "$target"
done
test ! -e /root/v0.1.0-lab.19-linux-amd64.tar.gz
test ! -e /root/vpskit-lab-v0.1.0-lab.19-zerossl-clean
test ! -e /root/.vpskit-cf-token
test ! -e /root/.vpskit-acme-email
echo 'LAB19_ZEROSSL_TEMP_CLEANUP=PASS'
