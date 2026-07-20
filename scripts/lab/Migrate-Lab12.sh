#!/usr/bin/env bash
set -Eeuo pipefail

archive='/root/v0.1.0-lab.12-linux-amd64.tar.gz'
lab_root='/root/vpskit-lab-v0.1.0-lab.12'
bundle="$lab_root/v0.1.0-lab.12"
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
install -d -m 0700 "$lab_root"
tar -xzf "$archive" -C "$lab_root"
chmod 0700 "$bundle"
chmod 0755 "$bundle/vpskit" "$bundle/sing-box" "$bundle/lego"
chmod 0644 "$bundle/release-manifest.json" "$bundle/release-manifest.sig"
"$bundle/vpskit" bundle verify --dir "$bundle"
"$bundle/vpskit" update self --bundle-dir "$bundle"
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.12'
systemctl is-active --quiet vpskit-certificate-renew.timer
systemctl is-active --quiet vpskit-sing-box.service
ss -ltnH | grep -q ':443 '
ss -lunH | grep -q ':443 '
echo 'LAB12_MIGRATION=PASS'
