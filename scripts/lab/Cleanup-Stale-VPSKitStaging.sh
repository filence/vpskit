#!/usr/bin/env bash
set -Eeuo pipefail

targets=(
    '/var/lib/vpskit/transactions/TX-20260718-060515-6489dc/staging'
    '/var/lib/vpskit/transactions/TX-20260718-063403-a61480/staging'
)
for target in "${targets[@]}"; do
    if [ ! -e "$target" ] && [ ! -L "$target" ]; then
        continue
    fi
    resolved="$(readlink -f -- "$target")"
    case "$resolved" in
        /var/lib/vpskit/transactions/TX-20260718-060515-6489dc/staging|/var/lib/vpskit/transactions/TX-20260718-063403-a61480/staging) ;;
        *) echo "CLEANUP_REFUSED unexpected_resolution=$target" >&2; exit 1 ;;
    esac
    rm -rf -- "$resolved"
done
for target in "${targets[@]}"; do
    test ! -e "$target"
done
echo 'STALE_VPSKIT_STAGING_CLEANUP=PASS'
