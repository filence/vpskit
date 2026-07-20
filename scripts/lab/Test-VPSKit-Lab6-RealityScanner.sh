#!/usr/bin/env bash
set -Eeuo pipefail

archive='/root/v0.1.0-lab.6-linux-amd64.tar.gz'
archive_sha256='bedd2a414bea9a542116ee72edde8a5cf4d50988c084e1ad6035b63c17f5463b'
lab_root='/root/vpskit-lab-v0.1.0-lab.6-scan'
bundle="$lab_root/v0.1.0-lab.6"
scanner_json="$lab_root/scanner.json"
amazon_preflight_json="$lab_root/amazon-preflight.json"
microsoft_error="$lab_root/microsoft-preflight.error"

cleanup() {
    case "$lab_root" in
        /root/vpskit-lab-v0.1.0-lab.6-scan) rm -rf -- "$lab_root" ;;
        *) echo 'LAB6_CLEANUP=REFUSED' >&2 ;;
    esac
    rm -f -- "$archive"
}
trap cleanup EXIT HUP INT TERM

test -f "$archive"
test ! -e "$lab_root"
printf '%s  %s\n' "$archive_sha256" "$archive" | sha256sum -c -
install -d -m 0700 "$lab_root"
tar -xzf "$archive" -C "$lab_root"
chmod 0700 "$bundle"
chmod 0755 "$bundle/vpskit" "$bundle/sing-box" "$bundle/lego"
chmod 0644 "$bundle/release-manifest.json" "$bundle/release-manifest.sig"

before_hash="$(sha256sum /etc/vpskit/generated/sing-box.json | awk '{print $1}')"
before_state="$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"

"$bundle/vpskit" bundle verify --dir "$bundle"
"$bundle/vpskit" reality scan \
    --sing-box "$bundle/sing-box" \
    --targets 'www.amazon.com,www.microsoft.com' >"$scanner_json"

python3 - "$scanner_json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding='utf-8'))
results = {entry['host']: entry for entry in payload['detail']['results']}
amazon = results['www.amazon.com']
microsoft = results['www.microsoft.com']
assert amazon['tls_candidate'] is True, amazon
assert amazon['reality_verified'] is True, amazon
assert microsoft['tls_candidate'] is True, microsoft
assert microsoft['reality_verified'] is False, microsoft
assert microsoft.get('reality_reason'), microsoft
print('REALITY_SCANNER=PASS amazon=tls+reality microsoft=tls-only-false-positive-rejected')
print('REALITY_SCANNER_AMAZON_LATENCY_MS=' + str(amazon.get('latency_ms', 0)))
print('REALITY_SCANNER_MICROSOFT_REASON_PRESENT=true')
PY

"$bundle/vpskit" preflight \
    --tcp-port 15443 \
    --udp-port 15443 \
    --reality-server-name www.amazon.com \
    --sing-box "$bundle/sing-box" >"$amazon_preflight_json"
python3 - "$amazon_preflight_json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding='utf-8'))
scan = payload['detail']['reality_scan']
assert payload['status'] == 'PASS', payload
assert scan['tls_candidate'] is True, scan
assert scan['reality_verified'] is True, scan
print('REALITY_PREFLIGHT_AMAZON=PASS')
PY

if "$bundle/vpskit" preflight \
    --tcp-port 15444 \
    --udp-port 15444 \
    --reality-server-name www.microsoft.com \
    --sing-box "$bundle/sing-box" >/dev/null 2>"$microsoft_error"; then
    echo 'REALITY_PREFLIGHT_MICROSOFT=UNEXPECTED_PASS' >&2
    exit 1
fi
grep -q 'Reality end-to-end preflight failed for www.microsoft.com' "$microsoft_error"
echo 'REALITY_PREFLIGHT_MICROSOFT=EXPECTED_REJECT'

after_hash="$(sha256sum /etc/vpskit/generated/sing-box.json | awk '{print $1}')"
after_state="$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
test "$before_hash" = "$after_hash"
test "$before_state" = "$after_state"
systemctl is-active --quiet vpskit-sing-box.service
/usr/local/bin/vpskit doctor >/dev/null
if compgen -G '/tmp/vpskit-reality-verify-*' >/dev/null; then
    echo 'REALITY_TEMP_RESIDUE=FAIL' >&2
    exit 1
fi

echo 'ACTIVE_DEPLOYMENT_UNCHANGED=PASS'
echo 'REALITY_TEMP_RESIDUE=PASS'
echo 'LAB6_REALITY_SCANNER_TEST=PASS'
