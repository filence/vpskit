#!/usr/bin/env bash
set -Eeuo pipefail

client_bundle='/root/vpskit-client-r0003-20260720-024924.zip'
bundle_record='/root/vpskit-lab32-final-client-bundle.path'
expected_sha256='2a9f07a3bce89de877d789ad2578ec0c614e23f7da49ad454a24b5a89b83c645'

test -f "$client_bundle"
test ! -L "$client_bundle"
test "$(sha256sum "$client_bundle" | awk '{print $1}')" = "$expected_sha256"
test -f "$bundle_record"
test ! -L "$bundle_record"
test "$(cat "$bundle_record")" = "$client_bundle"

rm -f -- "$client_bundle" "$bundle_record"
test ! -e "$client_bundle"
test ! -e "$bundle_record"
echo 'LAB32_CLIENT_EXPORT_REMOTE_CLEANUP=PASS removed=2'
