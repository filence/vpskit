#!/usr/bin/env bash
set -Eeuo pipefail

target='/var/lib/vpskit/transactions/TX-20260718-015748-cert-renew-0d051c/staging'
case "$target" in
    /var/lib/vpskit/transactions/TX-20260718-015748-cert-renew-0d051c/staging) ;;
    *) echo 'CLEANUP_REFUSED=unexpected_path' >&2; exit 1 ;;
esac
if [ -e "$target" ]; then
    resolved="$(readlink -f -- "$target")"
    test "$resolved" = "$target"
    rm -rf -- "$target"
fi
test ! -e "$target"
test "$(find /var/lib/vpskit/transactions -mindepth 2 -maxdepth 2 -type d -name staging | wc -l)" -eq 0
echo 'LAB8_STAGING_CLEANUP=PASS'
