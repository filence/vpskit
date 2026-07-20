#!/usr/bin/env bash
set -eu

lab_domain="${VPSKIT_LAB_DOMAIN:?VPSKIT_LAB_DOMAIN is required}"

service='vpskit-sing-box.service'
secret_env='/var/lib/vpskit/secrets/cloudflare.env'
pattern_file="$(mktemp /run/vpskit-token-pattern.XXXXXX)"
cleanup() {
    rm -f -- "$pattern_file"
}
trap cleanup EXIT HUP INT TERM

systemctl is-active --quiet "$service"
systemctl is-enabled --quiet "$service"
echo 'SERVICE_INITIAL=PASS'

systemctl restart "$service"
sleep 2
systemctl is-active --quiet "$service"
ss -ltnH | grep -q ':443 '
ss -lunH | grep -q ':443 '
echo 'SERVICE_RESTART=PASS'

runuser -u vpskit -- test -r /etc/vpskit/generated/sing-box.json
runuser -u vpskit -- test -r /var/lib/vpskit/certificates/hysteria2.crt
runuser -u vpskit -- test -r /var/lib/vpskit/certificates/hysteria2.key
echo 'SERVICE_USER_READ=PASS'

printf '%s\n' 'MANAGED_PATHS_BEGIN'
for managed_path in \
    /etc/vpskit \
    /etc/vpskit/generated \
    /etc/vpskit/generated/sing-box.json \
    /var/lib/vpskit \
    /var/lib/vpskit/certificates \
    /var/lib/vpskit/certificates/hysteria2.crt \
    /var/lib/vpskit/certificates/hysteria2.key \
    /var/lib/vpskit/secrets/cloudflare.env \
    /etc/vpskit/exports; do
    stat -c '%a %U:%G %n' "$managed_path"
done
printf '%s\n' 'MANAGED_PATHS_END'

printf '%s\n' 'UNIT_PROPERTIES_BEGIN'
systemctl show "$service" \
    -p User -p Group -p NoNewPrivileges -p PrivateTmp -p PrivateDevices \
    -p ProtectHome -p ProtectSystem -p RestrictAddressFamilies \
    -p CapabilityBoundingSet -p AmbientCapabilities
printf '%s\n' 'UNIT_PROPERTIES_END'
systemd-analyze security --no-pager "$service" | tail -n 1 || true

issuer_certificate="/var/lib/vpskit/lego/certificates/$lab_domain.issuer.crt"
openssl verify -CApath /etc/ssl/certs -untrusted "$issuer_certificate" /var/lib/vpskit/certificates/hysteria2.crt
cert_public="$(openssl x509 -in /var/lib/vpskit/certificates/hysteria2.crt -pubkey -noout | sha256sum | cut -d' ' -f1)"
key_public="$(openssl pkey -in /var/lib/vpskit/certificates/hysteria2.key -pubout 2>/dev/null | sha256sum | cut -d' ' -f1)"
test "$cert_public" = "$key_public"
echo 'CERTIFICATE_KEY_PAIR=PASS'
openssl x509 -in /var/lib/vpskit/certificates/hysteria2.crt -noout -dates -ext subjectAltName

test -s "$secret_env"
test "$(stat -c '%a %U:%G' "$secret_env")" = '600 root:root'
sed -n 's/^CF_DNS_API_TOKEN=//p' "$secret_env" > "$pattern_file"
chmod 0600 "$pattern_file"
test -s "$pattern_file"

token_file_hits=0
while IFS= read -r candidate; do
    if grep -Fq -f "$pattern_file" -- "$candidate"; then
        token_file_hits=$((token_file_hits + 1))
    fi
done < <(find /etc/vpskit /var/lib/vpskit /var/log/vpskit -type f ! -path "$secret_env" -print 2>/dev/null)
test "$token_file_hits" -eq 0

service_pid="$(systemctl show -p MainPID --value "$service")"
test "$service_pid" -gt 0
if tr '\0' '\n' < "/proc/$service_pid/environ" | grep -q '^CF_DNS_API_TOKEN='; then
    echo 'TOKEN_SERVICE_ENV=FAIL'
    exit 1
fi
journal_file="$(mktemp /run/vpskit-journal-scan.XXXXXX)"
journalctl -u "$service" --no-pager > "$journal_file"
if grep -Fq -f "$pattern_file" -- "$journal_file"; then
    rm -f -- "$journal_file"
    echo 'TOKEN_JOURNAL=FAIL'
    exit 1
fi
rm -f -- "$journal_file"
echo "TOKEN_RESIDUE=PASS scanned_file_hits=$token_file_hits service_env=clean journal=clean"

staging_count="$(find /var/lib/vpskit/transactions -type d -name staging 2>/dev/null | wc -l)"
test "$staging_count" -eq 0
committed_count=0
rolled_back_count=0
for transaction_record in /var/lib/vpskit/transactions/*/transaction.json; do
    test -f "$transaction_record" || continue
    transaction_status="$(sed -n 's/^[[:space:]]*"status":[[:space:]]*"\([^"]*\)".*/\1/p' "$transaction_record")"
    case "$transaction_status" in
        COMMITTED) committed_count=$((committed_count + 1)) ;;
        ROLLED_BACK) rolled_back_count=$((rolled_back_count + 1)) ;;
    esac
done
echo "TRANSACTIONS=PASS committed=$committed_count rolled_back=$rolled_back_count staging=$staging_count"

/usr/local/bin/vpskit status
/usr/local/bin/vpskit doctor
echo 'REMOTE_VERIFICATION=PASS'
