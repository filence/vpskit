#!/usr/bin/env bash
set -Eeuo pipefail

archive='/root/v0.1.0-lab.27-linux-amd64.tar.gz'
archive_sha256='9b9cf12e8996506ab07379ce3ca32c9b7ac52c28d760a58d7f3e129c4e457ad7'
release_parent='/root/vpskit-lab27'
bundle="$release_parent/v0.1.0-lab.27"
recovery_backup='BK-20260719-022959-pre-instance-a6d3e6'
old_delete_transaction='TX-20260719-023000-instance-92238a'

cleanup() {
    set +e
    rm -f -- "$archive"
    case "$release_parent" in
        /root/vpskit-lab27) rm -rf -- "$release_parent" ;;
        *) printf 'cleanup refused for unexpected release path\n' >&2 ;;
    esac
}
trap cleanup EXIT HUP INT TERM

test -f "$archive"
case "$release_parent" in
    /root/vpskit-lab27) rm -rf -- "$release_parent" ;;
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
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.27'

# Explicit restore intentionally restores the complete old recovery point,
# including its VPSKit binary. Reapply lab27 immediately after recovery.
/usr/local/bin/vpskit restore "$recovery_backup" --yes
"$bundle/vpskit" update self --bundle-dir "$bundle"
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.27'

set +e
cross_version_output="$(/usr/local/bin/vpskit rollback "$old_delete_transaction" --yes 2>&1)"
cross_version_code=$?
set -e
test "$cross_version_code" -ne 0
grep -Fq 'refusing cross-version instance rollback' <<<"$cross_version_output"
printf 'CROSS_VERSION_INSTANCE_ROLLBACK_GUARD=PASS\n'

python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
assert state['schema_version'] == 3
assert state['vpskit_version'] == 'v0.1.0-lab.27'
assert state['profile'] == 'balanced', state['profile']
assert state['reality']['enabled'] is True
assert state['hysteria2']['enabled'] is True
assert state['reality']['listen_port'] == 443
assert state['hysteria2']['listen_port'] == 443
print('LAB27_RECOVERY_STATE=PASS')
PY

/usr/local/bin/vpskit doctor
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-certificate-renew.timer
printf 'LAB27_RECOVERY_DEPLOY=PASS\n'
