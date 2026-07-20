#!/usr/bin/env bash
set -Eeuo pipefail

output="$(/usr/local/bin/vpskit reality scan --targets 'www.amazon.com,www.microsoft.com')"
printf '%s\n' "$output"
python3 -c '
import json, sys
payload = json.loads(sys.argv[1])
results = {item["host"]: item for item in payload["detail"]["results"]}
amazon = results["www.amazon.com"]
microsoft = results["www.microsoft.com"]
assert payload["status"] == "PASS"
assert amazon["tls_candidate"] is True
assert amazon["reality_verified"] is True
assert microsoft["reality_verified"] is False
assert microsoft.get("reality_reason")
' "$output"
if compgen -G '/tmp/vpskit-reality-verify-*' >/dev/null; then
    printf 'REALITY_SCANNER_TEMP_RESIDUE=FAIL\n' >&2
    exit 1
fi
systemctl is-active --quiet vpskit-sing-box.service
/usr/local/bin/vpskit doctor >/dev/null
printf 'REALITY_SCANNER_FINAL=PASS amazon=verified microsoft=false-positive-rejected\n'
