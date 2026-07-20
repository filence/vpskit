#!/usr/bin/env bash
set -Eeuo pipefail

archive='/root/v0.1.0-lab.31-linux-amd64.tar.gz'
archive_sha256='2cc3b8e94242053351e9df5e483f37425f9a39f57dacafa22b1a45bb6066ca38'
test_root='/root/vpskit-lab31-update'
bundle="$test_root/v0.1.0-lab.31"

cleanup() {
  set +e
  rm -rf -- "$test_root"
  rm -f -- "$archive"
}
trap cleanup EXIT HUP INT TERM

test -f "$archive"
printf '%s  %s\n' "$archive_sha256" "$archive" | sha256sum -c --strict
rm -rf -- "$test_root"
install -d -m 0700 "$test_root"
tar -xzf "$archive" -C "$test_root"
chmod 0700 "$bundle"
chmod 0755 "$bundle/vpskit" "$bundle/sing-box" "$bundle/xray" "$bundle/lego"
chmod 0644 "$bundle/release-manifest.json" "$bundle/release-manifest.sig" "$bundle/versions.lock"
"$bundle/vpskit" bundle verify --dir "$bundle" >/dev/null

/usr/local/bin/vpskit update self --bundle-dir "$bundle" >/dev/null
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.31'
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
echo 'LAB31_SELF_UPDATE=PASS'

core_output="$(/usr/local/bin/vpskit update core --bundle-dir "$bundle" --channel stable)"
grep -F -- 'ALREADY_CURRENT' <<<"$core_output" >/dev/null
python3 <<'PY'
import hashlib
import json
from pathlib import Path

with open('/etc/vpskit/versions.lock', encoding='utf-8') as stream:
    lock = json.load(stream)
assets = {item['id']: item for item in lock['assets']}
assert set(assets) == {'sing-box', 'xray', 'lego'}, assets
for asset_id, path in {
    'sing-box': '/usr/local/lib/vpskit/bin/sing-box',
    'xray': '/usr/local/lib/vpskit/bin/xray',
    'lego': '/usr/local/lib/vpskit/bin/lego',
}.items():
    digest = hashlib.sha256(Path(path).read_bytes()).hexdigest()
    assert digest == assets[asset_id]['binary_sha256'], (asset_id, digest)
with open('/var/lib/vpskit/state.json', encoding='utf-8') as stream:
    state = json.load(stream)
assert state['schema_version'] == 4, state
assert state['core']['version'] == '1.13.14', state
assert state['reality_core']['version'] == '26.3.27', state
PY
/usr/local/bin/vpskit doctor >/dev/null
echo 'LAB31_CORE_UPDATE_ALREADY_CURRENT=PASS'
echo 'LAB31_UPDATE_REGRESSION=PASS'
