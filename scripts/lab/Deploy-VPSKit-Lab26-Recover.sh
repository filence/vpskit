#!/usr/bin/env bash
set -Eeuo pipefail

archive='/root/v0.1.0-lab.26-linux-amd64.tar.gz'
archive_sha256='dea0ab13ca098f5309cd4d6718391ac74abc6860ed35325cb1b4f3965e8cc9c2'
release_parent='/root/vpskit-lab26'
bundle="$release_parent/v0.1.0-lab.26"
delete_transaction='TX-20260719-022532-instance-ca0331'

cleanup() {
    set +e
    rm -f -- "$archive"
    case "$release_parent" in
        /root/vpskit-lab26) rm -rf -- "$release_parent" ;;
        *) printf 'cleanup refused for unexpected release path\n' >&2 ;;
    esac
}
trap cleanup EXIT HUP INT TERM

test -f "$archive"
case "$release_parent" in
    /root/vpskit-lab26) rm -rf -- "$release_parent" ;;
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
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.26'
/usr/local/bin/vpskit rollback "$delete_transaction" --yes

python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
assert state['schema_version'] == 3
assert state['profile'] == 'balanced', state['profile']
assert state['reality']['enabled'] is True
assert state['hysteria2']['enabled'] is True
assert state['reality']['listen_port'] == 443
assert state['hysteria2']['listen_port'] == 443
print('LAB26_DELETE_ROLLBACK=PASS')
PY

/usr/local/bin/vpskit doctor
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-certificate-renew.timer
printf 'LAB26_RECOVERY_DEPLOY=PASS\n'
