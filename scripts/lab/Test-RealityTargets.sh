#!/usr/bin/env bash
set -eu

for target in www.microsoft.com www.apple.com www.cloudflare.com; do
    printf 'TARGET_BEGIN name=%s\n' "$target"
    if timeout 12 openssl s_client \
        -connect "${target}:443" \
        -servername "$target" \
        -verify_hostname "$target" \
        -verify_return_error \
        -brief </dev/null 2>&1; then
        printf 'TARGET_RESULT name=%s status=PASS\n' "$target"
    else
        printf 'TARGET_RESULT name=%s status=FAIL\n' "$target"
    fi
    printf 'TARGET_END name=%s\n' "$target"
done
