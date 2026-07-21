#!/usr/bin/env bash
set -Eeuo pipefail

readonly archive='/root/v0.1.1-lab.1-linux-amd64.tar.gz'
readonly archive_sha256='f2cb20111650a1af96e29a5c127863faccb70fe9613b3b76ca8783c9b55d1034'
readonly test_root='/root/vpskit-v011-upgrade'
readonly bundle="$test_root/v0.1.1-lab.1"

test -f "$archive"
printf '%s  %s\n' "$archive_sha256" "$archive" | sha256sum -c --strict >/dev/null
rm -rf -- "$test_root"
install -d -m 0700 "$test_root"
tar -xzf "$archive" -C "$test_root"
chmod 0700 "$bundle"
chmod 0755 "$bundle/vpskit" "$bundle/sing-box" "$bundle/xray" "$bundle/lego"
chmod 0644 "$bundle/release-manifest.json" "$bundle/release-manifest.sig" "$bundle/versions.lock"
"$bundle/vpskit" bundle verify --dir "$bundle" >"$test_root/bundle-verify.json"
printf 'V011_BUNDLE_VERIFY=PASS\n'

test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0'
source_schema="$(python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
assert state['schema_version'] in (5, 6), state
assert state['profile'] == 'balanced', state
assert state['config_revision'] == 1, state
if state['schema_version'] == 6:
    assert state['node']['node_id'] == 'node-main', state
    assert state['node']['display_name'] == 'JP', state
print(state['schema_version'])
PY
)"

before_hash="$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
"$bundle/vpskit" migrate check >"$test_root/migrate-check-pre.json"
"$bundle/vpskit" migrate plan >"$test_root/migrate-plan-pre.json"
after_hash="$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
test "$before_hash" = "$after_hash"
python3 - "$test_root/migrate-check-pre.json" "$test_root/migrate-plan-pre.json" "$source_schema" <<'PY'
import json
import sys

check = json.load(open(sys.argv[1], encoding='utf-8'))
plan = json.load(open(sys.argv[2], encoding='utf-8'))
source = int(sys.argv[3])
assert check['status'] == 'PASS' and check['detail']['source_schema'] == source, check
assert check['detail']['write_performed'] is False, check
assert plan['status'] == 'PASS' and plan['detail']['target_schema'] == 6, plan
assert plan['detail']['write_required'] is (source == 5), plan
PY
printf 'V011_MIGRATION_PREFLIGHT_ZERO_WRITE=PASS\n'

install -o root -g root -m 0755 /usr/local/bin/vpskit "$test_root/vpskit-v0.1.0"
install -o root -g root -m 0755 "$bundle/vpskit" /usr/local/bin/vpskit
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.1-lab.1'
/usr/local/bin/vpskit update self --bundle-dir "$bundle" >"$test_root/self-update-first.json"
self_update_transaction="$(python3 - "$test_root/self-update-first.json" <<'PY'
import json
import sys
print(json.load(open(sys.argv[1], encoding='utf-8'))['detail']['transaction_id'])
PY
)"
python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
assert state['schema_version'] == 6, state
assert state['vpskit_version'] == 'v0.1.1-lab.1', state
assert state['config_revision'] == 1, state
assert state['node']['display_name'] == 'JP', state
PY
/usr/local/bin/vpskit rollback "$self_update_transaction" --yes >"$test_root/self-update-rollback.json"
python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
assert state['schema_version'] == 6, state
assert state['vpskit_version'] == 'v0.1.0', state
assert state['node']['display_name'] == 'JP', state
assert state['config_revision'] == 1, state
PY
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.1-lab.1'
/usr/local/bin/vpskit update self --bundle-dir "$bundle" >"$test_root/self-update-final.json"
python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
assert state['schema_version'] == 6, state
assert state['vpskit_version'] == 'v0.1.1-lab.1', state
assert state['config_revision'] == 1, state
assert '# Renderer: mihomo/v1' in Path('/etc/vpskit/exports/mihomo.yaml').read_text(encoding='utf-8')
PY
printf 'V011_SELF_UPDATE_ROLLBACK_REAPPLY=PASS\n'

/usr/local/bin/vpskit node modify --display-name 'VPSKit-Rollback-Probe' --provider 'lab' --tags 'rollback,probe' >"$test_root/node-probe.json"
node_transaction="$(python3 - "$test_root/node-probe.json" <<'PY'
import json
import sys
print(json.load(open(sys.argv[1], encoding='utf-8'))['detail']['transaction_id'])
PY
)"
grep -q 'VPSKit-Rollback-Probe-Reality' /etc/vpskit/exports/mihomo.yaml
/usr/local/bin/vpskit rollback "$node_transaction" --yes >"$test_root/node-rollback.json"
python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
assert state['node']['display_name'] == 'JP', state
assert state['config_revision'] == 1, state
PY
/usr/local/bin/vpskit node modify --display-name 'VPSKit' --tags 'personal,stable' >"$test_root/node-final.json"
python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
assert state['node']['display_name'] == 'VPSKit', state
assert state['node']['tags'] == ['personal', 'stable'], state
assert state['config_revision'] == 2, state
mihomo = Path('/etc/vpskit/exports/mihomo.yaml').read_text(encoding='utf-8')
links = Path('/etc/vpskit/exports/share-links.txt').read_text(encoding='utf-8')
assert 'VPSKit-Reality' in mihomo and 'VPSKit-Hysteria2' in mihomo, mihomo
assert '#VPSKit-Reality' in links and '#VPSKit-Hysteria2' in links, links
PY
printf 'V011_NODE_METADATA_AND_ROLLBACK=PASS\n'

/usr/local/bin/vpskit system inspect >"$test_root/system-inspect.json"
/usr/local/bin/vpskit cleanup plan >"$test_root/cleanup-plan.json"
install -d -m 0700 "$test_root/support" "$test_root/client"
/usr/local/bin/vpskit support bundle --output-dir "$test_root/support" >"$test_root/support-result.json"
support_bundle="$(find "$test_root/support" -maxdepth 1 -type f -name 'vpskit-support-*.zip' -print -quit)"
test -n "$support_bundle"
python3 - "$support_bundle" <<'PY'
import json
import sys
import zipfile
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
secrets = json.loads(Path('/var/lib/vpskit/secrets/instances.json').read_text(encoding='utf-8'))
with zipfile.ZipFile(sys.argv[1]) as archive:
    names = set(archive.namelist())
    assert {'manifest.json', 'README.txt', 'state-redacted.json', 'system.json', 'renderers.json', 'transactions.json'} <= names, names
    payload = b'\n'.join(archive.read(name) for name in names)
    for forbidden in [
        state['connect_host'], state['domain'], state['reality_server_name'],
        state['reality']['public_key'], state['reality']['short_id'],
        secrets['reality_uuid'], secrets['reality_private_key'], secrets['hysteria2_password'],
    ]:
        if forbidden:
            assert forbidden.encode() not in payload, forbidden
PY
printf 'V011_SYSTEM_CLEANUP_SUPPORT=PASS\n'

/usr/local/bin/vpskit export --format bundle --output-dir "$test_root/client" >"$test_root/client-result.json"
client_bundle="$(find "$test_root/client" -maxdepth 1 -type f -name 'vpskit-client-*.zip' -print -quit)"
test -n "$client_bundle"
python3 - "$client_bundle" <<'PY'
import json
import sys
import zipfile

with zipfile.ZipFile(sys.argv[1]) as archive:
    manifest = json.loads(archive.read('manifest.json'))
    assert manifest['profile'] == 'balanced', manifest
    assert manifest['config_revision'] == 2, manifest
    assert '# Renderer: mihomo/v1' in archive.read('mihomo.yaml').decode(), manifest
PY

/usr/local/bin/vpskit status >"$test_root/status-final.json"
/usr/local/bin/vpskit doctor >"$test_root/doctor-final.json"
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-certificate-renew.timer
printf 'V011_CLIENT_BUNDLE=%s\n' "$client_bundle"
printf 'V011_REAL_MACHINE_ACCEPTANCE=PASS\n'
