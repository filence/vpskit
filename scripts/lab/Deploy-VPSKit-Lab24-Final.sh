#!/usr/bin/env bash
set -Eeuo pipefail

archive='/root/v0.1.0-lab.24-linux-amd64.tar.gz'
archive_sha256='f361bd38cb79333880bef01f3a7e2056b135811492ae8a134125c25bb1b8b770'
release_parent='/root/vpskit-lab24-final'
bundle="$release_parent/v0.1.0-lab.24"

cleanup() {
    set +e
    rm -f -- "$archive"
    rm -rf -- "$release_parent"
}
trap cleanup EXIT HUP INT TERM

test -f "$archive"
test ! -e "$release_parent"
printf '%s  %s\n' "$archive_sha256" "$archive" | sha256sum -c -
install -d -m 0700 "$release_parent"
tar -xzf "$archive" -C "$release_parent"
chmod 0700 "$bundle"
chmod 0755 "$bundle/vpskit" "$bundle/sing-box" "$bundle/lego"
chmod 0644 "$bundle/release-manifest.json" "$bundle/release-manifest.sig" "$bundle/versions.lock"

"$bundle/vpskit" bundle verify --dir "$bundle"
"$bundle/vpskit" update self --bundle-dir "$bundle"
test "$(/usr/local/bin/vpskit version)" = 'v0.1.0-lab.24'
/usr/local/bin/vpskit recover | tee /tmp/vpskit-lab24-recover.json
grep -q 'NOT_NEEDED' /tmp/vpskit-lab24-recover.json
/usr/local/bin/vpskit orphan scan | tee /tmp/vpskit-lab24-orphan.json
python3 - <<'PY'
import json

with open('/tmp/vpskit-lab24-orphan.json', encoding='utf-8') as handle:
    result = json.load(handle)
if result.get('status') != 'PASS':
    raise SystemExit('orphan scan did not pass')
detail = result.get('detail') or {}
if detail.get('needs_review') not in (False, None):
    raise SystemExit('orphan scan requires review')
PY
/usr/local/bin/vpskit doctor
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-certificate-renew.timer
ss -ltnH | grep -q ':443 '
ss -lunH | grep -q ':443 '
rm -f -- /tmp/vpskit-lab24-recover.json /tmp/vpskit-lab24-orphan.json
printf 'LAB24_FINAL=PASS\n'
