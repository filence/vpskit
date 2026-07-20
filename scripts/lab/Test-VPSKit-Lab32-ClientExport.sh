#!/usr/bin/env bash
set -Eeuo pipefail

archive='/root/v0.1.0-lab.32-linux-amd64.tar.gz'
archive_sha256='5d448b7c9163bd4a1e334f812cc1dfe7fba6a39f1dc38a85b94accca23ef737f'
test_root='/root/vpskit-lab32-client-export'
bundle="$test_root/v0.1.0-lab.32"
final_bundle_record='/root/vpskit-lab32-final-client-bundle.path'

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

baseline_secret="$(sha256sum /var/lib/vpskit/secrets/instances.json | awk '{print $1}')"
baseline_mihomo="$(sha256sum /etc/vpskit/exports/mihomo.yaml | awk '{print $1}')"
baseline_reality="$(sha256sum /etc/vpskit/exports/sing-box-reality.json | awk '{print $1}')"
baseline_hysteria2="$(sha256sum /etc/vpskit/exports/sing-box-hysteria2.json | awk '{print $1}')"
baseline_links="$(sha256sum /etc/vpskit/exports/share-links.txt | awk '{print $1}')"

/usr/local/bin/vpskit update self --bundle-dir "$bundle" >/dev/null
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.32'
/usr/local/bin/vpskit doctor >/dev/null
status_output="$(/usr/local/bin/vpskit status)"
python3 - "$status_output" <<'PY'
import json
import sys
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
status = json.loads(sys.argv[1])
assert state['schema_version'] in (4, 5), state['schema_version']
if state['schema_version'] == 4:
    assert 'config_revision' not in state, state.get('config_revision')
else:
    assert state['config_revision'] == 1, state.get('config_revision')
assert state['reality_server_name'] == 'www.amazon.com', state
assert status['detail']['state_schema'] == 5, status
assert status['detail']['config_revision'] == 1, status
PY
test "$(sha256sum /var/lib/vpskit/secrets/instances.json | awk '{print $1}')" = "$baseline_secret"
test "$(sha256sum /etc/vpskit/exports/mihomo.yaml | awk '{print $1}')" = "$baseline_mihomo"
test "$(sha256sum /etc/vpskit/exports/sing-box-reality.json | awk '{print $1}')" = "$baseline_reality"
test "$(sha256sum /etc/vpskit/exports/sing-box-hysteria2.json | awk '{print $1}')" = "$baseline_hysteria2"
test "$(sha256sum /etc/vpskit/exports/share-links.txt | awk '{print $1}')" = "$baseline_links"
echo 'LAB32_SCHEMA_MIGRATION_IN_MEMORY=PASS revision=1'

state_before_invalid="$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
set +e
/usr/local/bin/vpskit instance modify reality --reality-server-name 127.0.0.1 >"$test_root/invalid.stdout" 2>"$test_root/invalid.stderr"
invalid_exit=$?
set -e
test "$invalid_exit" -ne 0
test "$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')" = "$state_before_invalid"
echo 'LAB32_INVALID_TARGET_NO_WRITE=PASS'

cloudflare_output="$(/usr/local/bin/vpskit instance modify reality --reality-server-name www.cloudflare.com)"
python3 - "$cloudflare_output" <<'PY'
import json
import sys

value = json.loads(sys.argv[1])
detail = value['detail']
assert value['status'] == 'PASS', value
assert detail['config_revision'] == 2, detail
assert detail['client_update_required'] is True, detail
assert detail['exports_regenerated'] is True, detail
assert detail['changed_client_fields'] == ['reality.server_name'], detail
assert detail['reality_server_name'] == 'www.cloudflare.com', detail
assert detail['previous_backup_id'].startswith('BK-'), detail
PY
/usr/local/bin/vpskit doctor >/dev/null
test "$(sha256sum /var/lib/vpskit/secrets/instances.json | awk '{print $1}')" = "$baseline_secret"
test "$(sha256sum /etc/vpskit/exports/sing-box-hysteria2.json | awk '{print $1}')" = "$baseline_hysteria2"
test "$(sha256sum /etc/vpskit/exports/sing-box-reality.json | awk '{print $1}')" != "$baseline_reality"
python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
assert state['schema_version'] == 5, state
assert state['config_revision'] == 2, state
PY
echo 'LAB32_REALITY_TARGET_CHANGE=PASS revision=2'

amazon_output="$(/usr/local/bin/vpskit instance modify reality --reality-server-name www.amazon.com)"
python3 - "$amazon_output" <<'PY'
import json
import sys

value = json.loads(sys.argv[1])
detail = value['detail']
assert value['status'] == 'PASS', value
assert detail['config_revision'] == 3, detail
assert detail['client_update_required'] is True, detail
assert detail['changed_client_fields'] == ['reality.server_name'], detail
assert detail['reality_server_name'] == 'www.amazon.com', detail
assert detail['previous_backup_id'].startswith('BK-'), detail
PY
/usr/local/bin/vpskit doctor >/dev/null
test "$(sha256sum /var/lib/vpskit/secrets/instances.json | awk '{print $1}')" = "$baseline_secret"
test "$(sha256sum /etc/vpskit/exports/mihomo.yaml | awk '{print $1}')" = "$baseline_mihomo"
test "$(sha256sum /etc/vpskit/exports/sing-box-reality.json | awk '{print $1}')" = "$baseline_reality"
test "$(sha256sum /etc/vpskit/exports/sing-box-hysteria2.json | awk '{print $1}')" = "$baseline_hysteria2"
test "$(sha256sum /etc/vpskit/exports/share-links.txt | awk '{print $1}')" = "$baseline_links"
echo 'LAB32_REALITY_TARGET_RESTORED=PASS revision=3'

bundle_output="$(/usr/local/bin/vpskit export --format bundle)"
client_bundle="$(python3 -c 'import json,sys; value=json.loads(sys.argv[1]); assert value["status"] == "PASS", value; print(value["detail"]["bundle"])' "$bundle_output")"
test -f "$client_bundle"
test "$(stat -c '%a:%U:%G' "$client_bundle")" = '600:root:root'
python3 - "$client_bundle" <<'PY'
import hashlib
import json
import pathlib
import zipfile
import sys

path = pathlib.Path(sys.argv[1])
with zipfile.ZipFile(path) as archive:
    names = archive.namelist()
    assert names == [
        'mihomo.yaml',
        'sing-box-reality.json',
        'sing-box-hysteria2.json',
        'share-links.txt',
        'manifest.json',
        'README.txt',
    ], names
    assert all(not name.startswith('/') and '..' not in pathlib.PurePosixPath(name).parts for name in names), names
    manifest = json.loads(archive.read('manifest.json'))
    assert manifest['schema_version'] == 1, manifest
    assert manifest['profile'] == 'balanced', manifest
    assert manifest['config_revision'] == 3, manifest
    assert len(manifest['files']) == 4, manifest
    for entry in manifest['files']:
        content = archive.read(entry['name'])
        assert len(content) == entry['size'], entry
        assert hashlib.sha256(content).hexdigest() == entry['sha256'], entry
PY
printf '%s\n' "$client_bundle" >"$final_bundle_record"
chmod 0600 "$final_bundle_record"
echo "LAB32_CLIENT_BUNDLE=PASS revision=3 path=$client_bundle"
echo 'LAB32_REMOTE_REGRESSION=PASS'
