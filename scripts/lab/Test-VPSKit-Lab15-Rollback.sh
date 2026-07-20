#!/usr/bin/env bash
set -Eeuo pipefail

archive='/root/v0.1.0-lab.15-linux-amd64.tar.gz'
archive_sha256='000180d09dee079ab115d4dc7b45847e17987d07fdf6c91961392198887f5925'
lab_root='/root/vpskit-lab-v0.1.0-lab.15-rollback'
bundle="$lab_root/v0.1.0-lab.15"

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
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.14'
update_json="$(/usr/local/bin/vpskit update self --bundle-dir "$bundle")"
update_tx="$(python3 -c 'import json,sys; p=json.loads(sys.argv[1]); assert p["status"] == "PASS", p; print(p["detail"]["transaction_id"])' "$update_json")"
case "$update_tx" in TX-*) ;; *) echo 'UPDATE_TX_INVALID' >&2; exit 1 ;; esac
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.15'
systemctl is-enabled --quiet vpskit-certificate-renew.timer
systemctl is-active --quiet vpskit-certificate-renew.timer
echo 'UPDATE_TO_LAB15=PASS'

previous_backup_id="$(python3 -c 'import json,sys; p=json.loads(sys.argv[1]); print(p["detail"]["previous_backup_id"])' "$update_json")"
case "$previous_backup_id" in BK-*) ;; *) echo 'PREVIOUS_BACKUP_ID_INVALID' >&2; exit 1 ;; esac
manifest="/var/lib/vpskit/backups/$previous_backup_id/backup.json"
test -f "$manifest"
grep -q '"relative": "bin/vpskit"' "$manifest"
! grep -q 'cloudflare.env' "$manifest"
echo 'BINARY_BACKUP_MANIFEST=PASS'

/usr/local/bin/vpskit rollback "$update_tx" --yes
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.14'
systemctl is-enabled --quiet vpskit-certificate-renew.timer
systemctl is-active --quiet vpskit-certificate-renew.timer
systemctl is-active --quiet vpskit-sing-box.service
ss -ltnH | grep -q ':443 '
ss -lunH | grep -q ':443 '

python3 <<'PY'
import json
import pathlib

root = pathlib.Path('/var/lib/vpskit/transactions')
records = [json.loads(path.read_text(encoding='utf-8')) for path in sorted(root.glob('TX-*-restore-*/transaction.json'))]
assert records, 'no restore transaction record found'
assert records[-1]['status'] == 'COMMITTED', records[-1]
assert records[-1].get('command') == 'restore', records[-1]
print('ROLLBACK_TRANSACTION=PASS')
PY

test "$(find /var/lib/vpskit/transactions -mindepth 2 -maxdepth 2 -type d -name staging | wc -l)" -eq 0
echo 'BINARY_ROLLBACK=PASS'
