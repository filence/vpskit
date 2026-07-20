#!/usr/bin/env bash
set -Eeuo pipefail

archive='/root/v0.1.0-lab.28-linux-amd64.tar.gz'
archive_sha256='99a3f159eb49df10984ec90777edf7043aeffc82504bcdeae941f629a697bb24'
release_parent='/root/vpskit-lab28'
bundle="$release_parent/v0.1.0-lab.28"
recovery_backup='BK-20260719-023952-c272b4'

cleanup() {
    set +e
    rm -f -- "$archive"
    case "$release_parent" in
        /root/vpskit-lab28) rm -rf -- "$release_parent" ;;
        *) printf 'cleanup refused for unexpected release path\n' >&2 ;;
    esac
}
trap cleanup EXIT HUP INT TERM

test -f "$archive"
test -d "/var/lib/vpskit/backups/$recovery_backup"
case "$release_parent" in
    /root/vpskit-lab28) rm -rf -- "$release_parent" ;;
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
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.28'
/usr/local/bin/vpskit restore "$recovery_backup" --yes
# Full restore includes the backed-up lab27 binary; reapply lab28.
"$bundle/vpskit" update self --bundle-dir "$bundle"
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.28'

python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
assert state['schema_version'] == 3
assert state['vpskit_version'] == 'v0.1.0-lab.28'
assert state['profile'] == 'balanced'
assert state['reality']['enabled'] is True
assert state['hysteria2']['enabled'] is True
assert Path('/var/lib/vpskit/secrets/cloudflare.env').is_file()
assert Path('/var/lib/vpskit/secrets/acme.env').is_file()
print('LAB28_STAGED_CERT_RESTORE=PASS')
PY

/usr/local/bin/vpskit doctor
/usr/local/bin/vpskit cert status >/dev/null
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-certificate-renew.timer
printf 'LAB28_RECOVERY_DEPLOY=PASS\n'
