#!/usr/bin/env bash
set -Eeuo pipefail

archive='/root/v0.1.0-lab.29-linux-amd64.tar.gz'
archive_sha256='5db63c529adde6409ab7a5f63a15a7eca579ddc52d07d0e0b367889cc1bde0cd'
release_parent='/root/vpskit-lab29'
bundle="$release_parent/v0.1.0-lab.29"
recovery_backup='BK-20260719-023952-c272b4'

cleanup() {
    set +e
    rm -f -- "$archive"
    case "$release_parent" in
        /root/vpskit-lab29) rm -rf -- "$release_parent" ;;
        *) printf 'cleanup refused for unexpected release path\n' >&2 ;;
    esac
}
trap cleanup EXIT HUP INT TERM

test -f "$archive"
test -d "/var/lib/vpskit/backups/$recovery_backup"
case "$release_parent" in
    /root/vpskit-lab29) rm -rf -- "$release_parent" ;;
    *) exit 1 ;;
esac
printf '%s  %s\n' "$archive_sha256" "$archive" | sha256sum -c --strict
install -d -m 0700 "$release_parent"
tar -xzf "$archive" -C "$release_parent"
chmod 0700 "$bundle"
chmod 0755 "$bundle/vpskit" "$bundle/sing-box" "$bundle/lego"
chmod 0644 "$bundle/release-manifest.json" "$bundle/release-manifest.sig" "$bundle/versions.lock"

"$bundle/vpskit" bundle verify --dir "$bundle"
"$bundle/vpskit" update self --bundle-dir "$bundle"
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.29'
/usr/local/bin/vpskit restore "$recovery_backup" --yes
# Full restore includes the backed-up lab27 binary; reapply lab29.
"$bundle/vpskit" update self --bundle-dir "$bundle"
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.29'

python3 - <<'PY'
import json
import os
import stat
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
assert state['schema_version'] == 3
assert state['vpskit_version'] == 'v0.1.0-lab.29'
assert state['profile'] == 'balanced'
assert state['reality']['enabled'] is True
assert state['hysteria2']['enabled'] is True
assert stat.S_IMODE(os.stat('/var/lib/vpskit/certificates').st_mode) == 0o750
assert stat.S_IMODE(os.stat('/var/lib/vpskit/certificates/hysteria2.crt').st_mode) == 0o640
assert Path('/var/lib/vpskit/secrets/cloudflare.env').is_file()
assert Path('/var/lib/vpskit/secrets/acme.env').is_file()
print('LAB29_CERTIFICATE_RESTORE_PERMISSIONS=PASS')
PY

runuser -u vpskit -- test -r /var/lib/vpskit/certificates/hysteria2.crt
runuser -u vpskit -- test -r /var/lib/vpskit/certificates/hysteria2.key
/usr/local/bin/vpskit doctor
/usr/local/bin/vpskit cert status >/dev/null
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-certificate-renew.timer
printf 'LAB29_RECOVERY_DEPLOY=PASS\n'
