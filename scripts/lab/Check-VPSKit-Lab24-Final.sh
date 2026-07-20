#!/usr/bin/env bash
set -Eeuo pipefail

version_output="$(/usr/local/bin/vpskit version)"
printf '%s\n' "$version_output"
grep -Fq 'v0.1.0-lab.24' <<<"$version_output"

recover_output="$(/usr/local/bin/vpskit recover)"
printf '%s\n' "$recover_output"
grep -Fq 'NOT_NEEDED' <<<"$recover_output"

orphan_output="$(/usr/local/bin/vpskit orphan scan)"
printf '%s\n' "$orphan_output"
python3 -c 'import json,sys; result=json.loads(sys.argv[1]); detail=result.get("detail") or {}; assert result.get("status") == "PASS"; assert detail.get("needs_review") in (False, None)' "$orphan_output"

/usr/local/bin/vpskit doctor
printf 'LAB24_FINAL=PASS\n'
