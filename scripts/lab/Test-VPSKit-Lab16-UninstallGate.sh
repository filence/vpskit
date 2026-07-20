#!/usr/bin/env bash
set -Eeuo pipefail

archive='/root/v0.1.0-lab.16-linux-amd64.tar.gz'
archive_sha256='6ce50454845f4fdf25db6925fb5fdba63d37a871040594479dad8e8fbfeec456'
lab_root='/root/vpskit-lab-v0.1.0-lab.16-uninstall-gate'
bundle="$lab_root/v0.1.0-lab.16"

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
python3 -c 'import json,sys; p=json.loads(sys.argv[1]); assert p["status"] == "PASS", p' "$update_json"
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.16'
systemctl is-enabled --quiet vpskit-certificate-renew.timer
systemctl is-active --quiet vpskit-certificate-renew.timer

set +e
/usr/local/bin/vpskit uninstall >"$lab_root/uninstall-no-confirm.stdout" 2>"$lab_root/uninstall-no-confirm.stderr"
uninstall_exit=$?
set -e
test "$uninstall_exit" -ne 0
grep -q -- '--yes' "$lab_root/uninstall-no-confirm.stderr"
echo 'UNINSTALL_CONFIRMATION_GATE=PASS'

systemctl is-active --quiet vpskit-sing-box.service
ss -ltnH | grep -q ':443 '
ss -lunH | grep -q ':443 '
test "$(find /var/lib/vpskit/transactions -mindepth 2 -maxdepth 2 -type d -name staging | wc -l)" -eq 0
echo 'UNINSTALL_SAFE_GATE=PASS'
