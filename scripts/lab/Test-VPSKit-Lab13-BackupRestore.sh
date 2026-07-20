#!/usr/bin/env bash
set -Eeuo pipefail

archive='/root/v0.1.0-lab.13-linux-amd64.tar.gz'
archive_sha256='8bfda035670d7fdd5581d87dabdf6e11a4ffb63653274eed30a27e2f886877ec'
lab_root='/root/vpskit-lab-v0.1.0-lab.13-backup'
bundle="$lab_root/v0.1.0-lab.13"

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

"$bundle/vpskit" bundle verify --dir "$bundle"
"$bundle/vpskit" update self --bundle-dir "$bundle"
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.13'

before_config="$(sha256sum /etc/vpskit/generated/sing-box.json | awk '{print $1}')"
before_cert="$(sha256sum /var/lib/vpskit/certificates/hysteria2.crt | awk '{print $1}')"
before_key="$(sha256sum /var/lib/vpskit/certificates/hysteria2.key | awk '{print $1}')"
backup_json="$(/usr/local/bin/vpskit backup)"
backup_id="$(python3 -c 'import json,sys; p=json.loads(sys.argv[1]); assert p["status"] == "PASS", p; print(p["detail"]["backup_id"])' "$backup_json")"
case "$backup_id" in BK-*) ;; *) echo 'BACKUP_ID_INVALID' >&2; exit 1 ;; esac
test -f "/var/lib/vpskit/backups/$backup_id/backup.json"
echo 'BACKUP_CREATE=PASS'

set +e
/usr/local/bin/vpskit restore "$backup_id" >"$lab_root/restore-no-confirm.stdout" 2>"$lab_root/restore-no-confirm.stderr"
no_confirm_exit=$?
set -e
test "$no_confirm_exit" -ne 0
grep -q -- '--yes' "$lab_root/restore-no-confirm.stderr"
echo 'RESTORE_CONFIRMATION_GATE=PASS'

printf '\n' >> /etc/vpskit/generated/sing-box.json
test "$(sha256sum /etc/vpskit/generated/sing-box.json | awk '{print $1}')" != "$before_config"
/usr/local/bin/vpskit restore "$backup_id" --yes

test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.13'
test "$(sha256sum /etc/vpskit/generated/sing-box.json | awk '{print $1}')" = "$before_config"
test "$(sha256sum /var/lib/vpskit/certificates/hysteria2.crt | awk '{print $1}')" = "$before_cert"
test "$(sha256sum /var/lib/vpskit/certificates/hysteria2.key | awk '{print $1}')" = "$before_key"
test "$(stat -c '%U:%G %a' /var/lib/vpskit/certificates/hysteria2.crt)" = 'root:vpskit 640'
test "$(stat -c '%U:%G %a' /var/lib/vpskit/certificates/hysteria2.key)" = 'root:vpskit 640'
systemctl is-enabled --quiet vpskit-certificate-renew.timer
systemctl is-active --quiet vpskit-certificate-renew.timer
systemctl is-active --quiet vpskit-sing-box.service
ss -ltnH | grep -q ':443 '
ss -lunH | grep -q ':443 '

python3 <<'PY'
import hashlib
import json
import pathlib

state = json.loads(pathlib.Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
config = pathlib.Path('/etc/vpskit/generated/sing-box.json').read_bytes()
assert state['config_sha256'] == hashlib.sha256(config).hexdigest(), state
root = pathlib.Path('/var/lib/vpskit/transactions')
records = [json.loads(path.read_text(encoding='utf-8')) for path in sorted(root.glob('TX-*-restore-*/transaction.json'))]
assert records, 'no restore transaction record found'
assert records[-1]['status'] == 'COMMITTED', records[-1]
print('RESTORE_TRANSACTION=PASS')
PY

test "$(find /var/lib/vpskit/transactions -mindepth 2 -maxdepth 2 -type d -name staging | wc -l)" -eq 0
echo 'BACKUP_RESTORE=PASS'
