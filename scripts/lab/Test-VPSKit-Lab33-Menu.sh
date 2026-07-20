#!/usr/bin/env bash
set -Eeuo pipefail

readonly bootstrap_path='/root/vpskit-bootstrap-lab33-install.sh'
readonly archive_path='/root/v0.1.0-lab.33-linux-amd64.tar.gz'
readonly test_root='/root/vpskit-lab33-menu-smoke'
readonly bundle="$test_root/v0.1.0-lab.33"

cleanup() {
    rm -f -- "$bootstrap_path" "$archive_path"
    rm -rf -- "$test_root"
}
trap cleanup EXIT HUP INT TERM

[ -f "$bootstrap_path" ] || { printf 'LAB33_MENU=FAIL reason=missing-bootstrap\n' >&2; exit 1; }
[ -f "$archive_path" ] || { printf 'LAB33_MENU=FAIL reason=missing-archive\n' >&2; exit 1; }
[ ! -e "$test_root" ] || { printf 'LAB33_MENU=FAIL reason=test-root-exists\n' >&2; exit 1; }

bash "$bootstrap_path" --archive "$archive_path" --verify-only >/dev/null
printf 'LAB33_BOOTSTRAP_VERIFY=PASS\n'

mkdir -m 0700 -- "$test_root"
tar -xzf "$archive_path" -C "$test_root"
"$bundle/vpskit" bundle verify --dir "$bundle" >/dev/null
"$bundle/vpskit" update self --bundle-dir "$bundle" >/dev/null
[ "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.33' ]
/usr/local/bin/vpskit doctor >"$test_root/doctor.json"
grep -Fq '"status": "PASS"' "$test_root/doctor.json"
printf 'LAB33_UPDATE_DOCTOR=PASS\n'

printf '1\n0\n' | /usr/local/bin/vpskit menu >"$test_root/menu-status.txt"
grep -Fq 'VPSKit 中文管理菜单' "$test_root/menu-status.txt"
grep -Fq '"command": "status"' "$test_root/menu-status.txt"
grep -Fq '"status": "PASS"' "$test_root/menu-status.txt"

printf '8\n0\n' | /usr/local/bin/vpskit menu >"$test_root/menu-cert.txt"
grep -Fq '"command": "cert status"' "$test_root/menu-cert.txt"
grep -Fq '"status": "PASS"' "$test_root/menu-cert.txt"

printf '5\nwww.amazon.com\n0\n' | /usr/local/bin/vpskit menu >"$test_root/menu-scan.txt"
grep -Fq '"command": "reality scan"' "$test_root/menu-scan.txt"
grep -Fq '"reality_verified": true' "$test_root/menu-scan.txt"
printf 'LAB33_MENU_READ_ONLY=PASS\n'

state_hash_before="$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
printf '6\nwww.cloudflare.com\napply\n0\n' | /usr/local/bin/vpskit menu >"$test_root/menu-cancel.txt"
grep -Fq '操作已取消，未修改VPSKit' "$test_root/menu-cancel.txt"
state_hash_after="$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
[ "$state_hash_before" = "$state_hash_after" ]
printf 'LAB33_MENU_CANCEL_ZERO_WRITE=PASS\n'
