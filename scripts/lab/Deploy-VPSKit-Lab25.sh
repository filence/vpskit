#!/usr/bin/env bash
set -Eeuo pipefail

archive='/root/v0.1.0-lab.25-linux-amd64.tar.gz'
archive_sha256='2f6107ff903f8469c61ad214561999219e467929d7838dd055c2f80a16f01e9f'
release_parent='/root/vpskit-lab25'
bundle="$release_parent/v0.1.0-lab.25"

cleanup() {
    set +e
    rm -f -- "$archive"
    case "$release_parent" in
        /root/vpskit-lab25) rm -rf -- "$release_parent" ;;
        *) printf 'cleanup refused for unexpected release path\n' >&2 ;;
    esac
}
trap cleanup EXIT HUP INT TERM

test -f "$archive"
case "$release_parent" in
    /root/vpskit-lab25) rm -rf -- "$release_parent" ;;
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
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.25'

python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
assert state['schema_version'] == 3, state['schema_version']
assert state['profile'] == 'balanced', state['profile']
assert state['reality']['enabled'] is True
assert state['hysteria2']['enabled'] is True
assert state['vpskit_version'] == 'v0.1.0-lab.25'
print('LAB25_SCHEMA_MIGRATION=PASS')
PY

/usr/local/lib/vpskit/bin/sing-box check -c /etc/vpskit/generated/sing-box.json
/usr/local/bin/vpskit doctor
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-certificate-renew.timer
printf 'LAB25_DEPLOY=PASS\n'
