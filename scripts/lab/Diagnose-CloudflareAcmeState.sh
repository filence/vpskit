#!/usr/bin/env bash
set -Eeuo pipefail

lab_domain="${VPSKIT_LAB_DOMAIN:?VPSKIT_LAB_DOMAIN is required}"
lab_zone="${VPSKIT_LAB_ZONE:?VPSKIT_LAB_ZONE is required}"
lab_ipv4="${VPSKIT_LAB_IPV4:?VPSKIT_LAB_IPV4 is required}"

token_file='/root/.vpskit-cf-token'
test -f "$token_file"
token="$(tr -d '\r\n' < "$token_file")"
test -n "$token"
tmp="$(mktemp -d /var/tmp/vpskit-cf-diagnose.XXXXXX)"
cleanup() { rm -rf -- "$tmp"; unset token; }
trap cleanup EXIT HUP INT TERM

zone_code="$(curl -sS -o "$tmp/zone.json" -w '%{http_code}' -H "Authorization: Bearer $token" -H 'Accept: application/json' "https://api.cloudflare.com/client/v4/zones?name=$lab_zone")"
zone_id="$(python3 - "$tmp/zone.json" <<'PY'
import json
import sys
print(json.load(open(sys.argv[1], encoding='utf-8'))['result'][0]['id'])
PY
)"
record_code="$(curl -sS -o "$tmp/a.json" -w '%{http_code}' -H "Authorization: Bearer $token" -H 'Accept: application/json' "https://api.cloudflare.com/client/v4/zones/$zone_id/dns_records?type=A&name=$lab_domain")"
txt_code="$(curl -sS -o "$tmp/txt.json" -w '%{http_code}' -H "Authorization: Bearer $token" -H 'Accept: application/json' "https://api.cloudflare.com/client/v4/zones/$zone_id/dns_records?type=TXT&name=_acme-challenge.$lab_domain")"
python3 - "$tmp/zone.json" "$tmp/a.json" "$tmp/txt.json" "$zone_code" "$record_code" "$txt_code" "$lab_ipv4" <<'PY'
import json
import pathlib
import sys

zone, a_record, txt_record, zone_code, a_code, txt_code, expected_ipv4 = sys.argv[1:]
zone_data = json.loads(pathlib.Path(zone).read_text())
a_data = json.loads(pathlib.Path(a_record).read_text())
txt_data = json.loads(pathlib.Path(txt_record).read_text())
assert zone_data.get('success') is True, zone_data.get('errors')
assert a_data.get('success') is True, a_data.get('errors')
assert txt_data.get('success') is True, txt_data.get('errors')
a_records = a_data.get('result', [])
txt_records = txt_data.get('result', [])
assert len(a_records) == 1, a_records
assert a_records[0].get('content') == expected_ipv4, a_records[0]
assert a_records[0].get('proxied') is False, a_records[0]
print(f'CLOUDFLARE_API=PASS zone_http={zone_code} a_http={a_code} txt_http={txt_code}')
print(f'A_RECORD=PASS content={a_records[0]["content"]} proxied={a_records[0]["proxied"]}')
print(f'ACME_TXT_RECORDS={len(txt_records)}')
PY
