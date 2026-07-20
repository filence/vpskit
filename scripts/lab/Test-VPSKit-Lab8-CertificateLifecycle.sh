#!/usr/bin/env bash
set -Eeuo pipefail

archive='/root/v0.1.0-lab.8-linux-amd64.tar.gz'
archive_sha256='28c34d761cbc94254335127ac1c3033ba3dec46eed763df9fc7bffae7379a20a'
lab_root='/root/vpskit-lab-v0.1.0-lab.8-cert'
bundle="$lab_root/v0.1.0-lab.8"
skip_json="$lab_root/renew-skip.json"
rollback_stderr="$lab_root/renew-rollback.stderr"
rollback_stdout="$lab_root/renew-rollback.stdout"
lego='/usr/local/lib/vpskit/bin/lego'
lego_backup='/usr/local/lib/vpskit/bin/lego.vpskit-lab8-backup'
fake_lego='/usr/local/lib/vpskit/bin/lego.vpskit-lab8-fake'

cleanup() {
    set +e
    unset VPSKIT_CF_DNS_API_TOKEN || true
    rm -f -- "$fake_lego"
    if [ -f "$lego_backup" ]; then
        mv -- "$lego_backup" "$lego"
        chmod 0755 "$lego"
    fi
    systemctl reset-failed vpskit-sing-box.service >/dev/null 2>&1 || true
    systemctl restart vpskit-sing-box.service >/dev/null 2>&1 || true
    case "$lab_root" in
        /root/vpskit-lab-v0.1.0-lab.8-cert) rm -rf -- "$lab_root" ;;
        *) echo 'LAB8_CLEANUP=REFUSED' >&2 ;;
    esac
    rm -f -- "$archive"
}
trap cleanup EXIT HUP INT TERM

test -f "$archive"
test ! -e "$lab_root"
test -f "$lego"
test ! -e "$lego_backup"
printf '%s  %s\n' "$archive_sha256" "$archive" | sha256sum -c -
install -d -m 0700 "$lab_root"
tar -xzf "$archive" -C "$lab_root"
chmod 0700 "$bundle"
chmod 0755 "$bundle/vpskit" "$bundle/sing-box" "$bundle/lego"
chmod 0644 "$bundle/release-manifest.json" "$bundle/release-manifest.sig"

before_cert="$(sha256sum /var/lib/vpskit/certificates/hysteria2.crt | awk '{print $1}')"
before_key="$(sha256sum /var/lib/vpskit/certificates/hysteria2.key | awk '{print $1}')"
before_config="$(sha256sum /etc/vpskit/generated/sing-box.json | awk '{print $1}')"
before_state="$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
test "$(stat -c '%U:%G %a' /var/lib/vpskit/certificates/hysteria2.crt)" = 'root:vpskit 640'
test "$(stat -c '%U:%G %a' /var/lib/vpskit/certificates/hysteria2.key)" = 'root:vpskit 640'

"$bundle/vpskit" bundle verify --dir "$bundle"
"$bundle/vpskit" cert status
"$bundle/vpskit" cert renew >"$skip_json"
python3 - "$skip_json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding='utf-8'))
assert payload['status'] == 'PASS', payload
assert payload['detail']['result'] == 'SKIPPED', payload
assert payload['detail']['renewal_window_days'] == 30, payload
print('CERT_RENEWAL_NOT_DUE=PASS')
PY

mv -- "$lego" "$lego_backup"
cat >"$fake_lego" <<'FAKE_LEGO'
#!/usr/bin/env bash
set -Eeuo pipefail
staging=''
domain=''
while [ "$#" -gt 0 ]; do
    case "$1" in
        --path|--domains)
            test "$#" -ge 2
            if [ "$1" = '--path' ]; then staging="$2"; else domain="$2"; fi
            shift 2
            ;;
        *) shift ;;
    esac
done
test -n "$staging"
test -n "$domain"
mkdir -p "$staging/certificates"
cp /var/lib/vpskit/certificates/hysteria2.crt "$staging/certificates/$domain.crt"
printf '\n' >>"$staging/certificates/$domain.crt"
cp /var/lib/vpskit/certificates/hysteria2.key "$staging/certificates/$domain.key"
sleep 2
FAKE_LEGO
chmod 0755 "$fake_lego"
mv -- "$fake_lego" "$lego"

set +e
"$bundle/vpskit" cert renew --force >"$rollback_stdout" 2>"$rollback_stderr" &
renew_pid=$!
activation_observed=0
for _ in $(seq 1 1800); do
    current_cert="$(sha256sum /var/lib/vpskit/certificates/hysteria2.crt | awk '{print $1}')"
    if [ "$current_cert" != "$before_cert" ]; then
        activation_observed=1
        for _ in $(seq 1 100); do
            if systemctl is-active --quiet vpskit-sing-box.service; then
                systemctl stop vpskit-sing-box.service || true
                break
            fi
            if ! kill -0 "$renew_pid" 2>/dev/null; then
                break
            fi
            sleep 0.1
        done
        break
    fi
    if ! kill -0 "$renew_pid" 2>/dev/null; then
        break
    fi
    sleep 0.2
done
wait "$renew_pid"
renew_exit=$?
set -e
test "$activation_observed" -eq 1
test "$renew_exit" -ne 0
grep -q 'health check after certificate renewal failed' "$rollback_stderr"
echo 'CERT_RENEWAL_FORCED_ROLLBACK=EXPECTED_FAILURE'

after_cert="$(sha256sum /var/lib/vpskit/certificates/hysteria2.crt | awk '{print $1}')"
after_key="$(sha256sum /var/lib/vpskit/certificates/hysteria2.key | awk '{print $1}')"
after_config="$(sha256sum /etc/vpskit/generated/sing-box.json | awk '{print $1}')"
after_state="$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
test "$before_cert" = "$after_cert"
test "$before_key" = "$after_key"
test "$before_config" = "$after_config"
test "$before_state" = "$after_state"

python3 <<'PY'
import json
import pathlib

root = pathlib.Path('/var/lib/vpskit/transactions')
records = []
for path in sorted(root.glob('TX-*-cert-renew-*/transaction.json')):
    records.append(json.loads(path.read_text(encoding='utf-8')))
assert records, 'no certificate renewal transaction record found'
assert records[-1]['status'] == 'ROLLED_BACK', records[-1]
print('CERT_RENEWAL_ROLLBACK_RECORD=PASS')
PY

systemctl is-active --quiet vpskit-sing-box.service
/usr/local/bin/vpskit doctor >/dev/null
test "$(find /var/lib/vpskit/transactions -mindepth 2 -maxdepth 2 -type d -name staging | wc -l)" -eq 0
echo 'CERTIFICATE_FILES_UNCHANGED=PASS'
echo 'ACTIVE_SERVICE_UNCHANGED=PASS'
echo 'CERTIFICATE_RENEWAL_LAB8=PASS'
