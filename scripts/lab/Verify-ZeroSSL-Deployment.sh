#!/usr/bin/env bash
set -Eeuo pipefail

acme_file='/var/lib/vpskit/secrets/acme.env'
state_file='/var/lib/vpskit/state.json'
certificate='/var/lib/vpskit/certificates/hysteria2.crt'
test -f "$acme_file"
test -f "$state_file"
test -f "$certificate"
test "$(stat -c '%a %U:%G' "$acme_file")" = '600 root:root'
grep -qx 'ACME_SERVER=https://acme.zerossl.com/v2/DV90' "$acme_file"
grep -q '^ACME_EMAIL=.' "$acme_file"
grep -q '^ACME_EAB_KID=.' "$acme_file"
grep -q '^ACME_EAB_HMAC=.' "$acme_file"
python3 - "$state_file" <<'PY'
import json
import sys
state = json.load(open(sys.argv[1], encoding='utf-8'))
assert state['hysteria2']['certificate_authority'] == 'https://acme.zerossl.com/v2/DV90'
print('STATE_CA=ZeroSSL')
PY
issuer="$(openssl x509 -in "$certificate" -noout -issuer | sed 's/^issuer=//')"
subject="$(openssl x509 -in "$certificate" -noout -subject | sed 's/^subject=//')"
printf 'CERTIFICATE_ISSUER=%s\nCERTIFICATE_SUBJECT=%s\n' "$issuer" "$subject"
staging_count="$(find /var/lib/vpskit/transactions -type d -name staging 2>/dev/null | wc -l)"
printf 'STAGING_COUNT=%s\n' "$staging_count"
test "$staging_count" -eq 0
echo 'ZEROSSL_DEPLOYMENT=PASS eab_present=true staging=0'
