#!/usr/bin/env bash
set -eu

lab_domain="${VPSKIT_LAB_DOMAIN:?VPSKIT_LAB_DOMAIN is required}"

managed='/var/lib/vpskit/certificates/hysteria2.crt'
lego_cert="/var/lib/vpskit/lego/certificates/$lab_domain.crt"
lego_issuer="/var/lib/vpskit/lego/certificates/$lab_domain.issuer.crt"

for candidate in "$managed" "$lego_cert" "$lego_issuer"; do
    test -f "$candidate"
    blocks="$(grep -c -- '-----BEGIN CERTIFICATE-----' "$candidate")"
    size="$(stat -c '%s' "$candidate")"
    digest="$(sha256sum "$candidate" | cut -d' ' -f1)"
    printf 'certificate_file=%s blocks=%s size=%s sha256=%s\n' "$candidate" "$blocks" "$size" "$digest"
done

test "$(sha256sum "$managed" | cut -d' ' -f1)" = "$(sha256sum "$lego_cert" | cut -d' ' -f1)"
openssl verify -CApath /etc/ssl/certs -untrusted "$lego_issuer" "$managed"
echo 'CERTIFICATE_CHAIN_DIAGNOSIS=PASS managed_copy=exact issuer_validation=pass'
