#!/usr/bin/env bash
set -Eeuo pipefail

targets=(
    '/var/lib/vpskit/backups/BK-20260718-023352-pre-update-7fc696'
    '/var/lib/vpskit/backups/BK-20260718-023355-81811e'
    '/var/lib/vpskit/backups/BK-20260718-023741-pre-update-aa3eab'
    '/var/lib/vpskit/backups/BK-20260718-023744-256ca5'
    '/var/lib/vpskit/backups/BK-20260718-024028-pre-update-941e28'
)
for target in "${targets[@]}"; do
    if [ ! -e "$target" ] && [ ! -L "$target" ]; then
        continue
    fi
    test "$(readlink -f -- "$target")" = "$target"
    rm -rf -- "$target"
done
for target in "${targets[@]}"; do
    test ! -e "$target"
done
echo 'TEST_BACKUPS_CLEANUP=PASS'
