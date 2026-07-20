#!/usr/bin/env bash
set -Eeuo pipefail

archive='/root/v0.1.0-lab.9-linux-amd64.tar.gz'
archive_sha256='ee9513462a0c9ebbdfef4c26f2b5011714f6ebe4f422b31cde8b71f0d783fbd4'
lab_root='/root/vpskit-lab-v0.1.0-lab.9-update'
bundle="$lab_root/v0.1.0-lab.9"
fake_bin="$lab_root/fake-bin"
fake_systemctl="$fake_bin/systemctl"
failure_stdout="$lab_root/update-rollback.stdout"
failure_stderr="$lab_root/update-rollback.stderr"

cleanup() {
    set +e
    systemctl reset-failed vpskit-sing-box.service >/dev/null 2>&1 || true
    systemctl restart vpskit-sing-box.service >/dev/null 2>&1 || true
    rm -rf -- "$lab_root"
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

before_config="$(sha256sum /etc/vpskit/generated/sing-box.json | awk '{print $1}')"
before_state="$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
before_cert="$(sha256sum /var/lib/vpskit/certificates/hysteria2.crt | awk '{print $1}')"
before_key="$(sha256sum /var/lib/vpskit/certificates/hysteria2.key | awk '{print $1}')"

"$bundle/vpskit" bundle verify --dir "$bundle"
"$bundle/vpskit" update self --bundle-dir "$bundle"

test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.9'
systemctl is-enabled --quiet vpskit-certificate-renew.timer
systemctl is-active --quiet vpskit-certificate-renew.timer
systemctl is-active --quiet vpskit-sing-box.service
ss -ltnH | grep -q ':443 '
ss -lunH | grep -q ':443 '

after_config="$(sha256sum /etc/vpskit/generated/sing-box.json | awk '{print $1}')"
after_state="$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
after_cert="$(sha256sum /var/lib/vpskit/certificates/hysteria2.crt | awk '{print $1}')"
after_key="$(sha256sum /var/lib/vpskit/certificates/hysteria2.key | awk '{print $1}')"
test "$before_config" = "$after_config"
test "$before_cert" = "$after_cert"
test "$before_key" = "$after_key"
test "$before_state" != "$after_state"
test "$(stat -c '%U:%G %a' /var/lib/vpskit/certificates/hysteria2.crt)" = 'root:vpskit 640'
test "$(stat -c '%U:%G %a' /var/lib/vpskit/certificates/hysteria2.key)" = 'root:vpskit 640'

/usr/local/bin/vpskit cert status
python3 <<'PY'
import json
import pathlib

state = json.loads(pathlib.Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
assert state['vpskit_version'] == 'v0.1.0-lab.9', state
ownership = json.loads(pathlib.Path('/var/lib/vpskit/ownership.json').read_text(encoding='utf-8'))
assert '/etc/systemd/system/vpskit-certificate-renew.timer' in ownership['files'], ownership
assert 'vpskit-certificate-renew.timer' in ownership['services'], ownership
print('UPDATE_MIGRATION_STATE=PASS')
PY

install -d -m 0700 "$fake_bin"
cat >"$fake_systemctl" <<'FAKE_SYSTEMCTL'
#!/usr/bin/env bash
set -Eeuo pipefail
marker='/root/vpskit-lab-v0.1.0-lab.9-update/restart-failed-once'
if [ "${1:-}" = 'restart' ] && [ "${2:-}" = 'vpskit-sing-box.service' ] && [ ! -e "$marker" ]; then
    : >"$marker"
    echo 'controlled update restart failure' >&2
    exit 1
fi
exec /usr/bin/systemctl "$@"
FAKE_SYSTEMCTL
chmod 0755 "$fake_systemctl"

set +e
PATH="$fake_bin:$PATH" "$bundle/vpskit" update self --bundle-dir "$bundle" >"$failure_stdout" 2>"$failure_stderr"
rollback_exit=$?
set -e
test "$rollback_exit" -ne 0
grep -q 'service restart after update failed' "$failure_stderr"

test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.9'
systemctl is-enabled --quiet vpskit-certificate-renew.timer
systemctl is-active --quiet vpskit-certificate-renew.timer
systemctl is-active --quiet vpskit-sing-box.service
ss -ltnH | grep -q ':443 '
ss -lunH | grep -q ':443 '
test "$(sha256sum /etc/vpskit/generated/sing-box.json | awk '{print $1}')" = "$after_config"
test "$(sha256sum /var/lib/vpskit/certificates/hysteria2.crt | awk '{print $1}')" = "$after_cert"
test "$(sha256sum /var/lib/vpskit/certificates/hysteria2.key | awk '{print $1}')" = "$after_key"

python3 <<'PY'
import json
import pathlib

root = pathlib.Path('/var/lib/vpskit/transactions')
records = [json.loads(path.read_text(encoding='utf-8')) for path in sorted(root.glob('TX-*-update-self-*/transaction.json'))]
assert records, 'no update transaction record found'
assert records[-1]['status'] == 'ROLLED_BACK', records[-1]
print('UPDATE_ROLLBACK_RECORD=PASS')
PY

test "$(find /var/lib/vpskit/transactions -mindepth 2 -maxdepth 2 -type d -name staging | wc -l)" -eq 0
echo 'UPDATE_MIGRATION=PASS'
echo 'UPDATE_ROLLBACK=PASS'
