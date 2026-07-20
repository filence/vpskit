#!/usr/bin/env bash
set -euo pipefail

release_id='v0.1.0-lab.21'
release_archive="/root/${release_id}-linux-amd64.tar.gz"
release_sha256='e1dd9c0e3a19ec7b935f98bb3be246a3f05fa4c30f412ee56174c38e0a2866b0'
rotation_id='v0.1.0-lab.21-next-key'
rotation_archive="/root/${rotation_id}-linux-amd64.tar.gz"
rotation_sha256='95db37bcb09137e57d7a74fd3bd4867ecac7e2eb51aa175ecd8cf643069a039d'
lab_root='/root/vpskit-lab21-supply-chain'
bundle="$lab_root/$release_id"
rotation_bundle="$lab_root/$rotation_id"
tampered_bundle="$lab_root/tampered"
evidence="$lab_root/evidence"
orphan_fixture='/usr/local/lib/vpskit/bin/vpskit-orphan-fixture'

cleanup_fixture() {
    rm -f -- "$orphan_fixture"
}
trap cleanup_fixture EXIT HUP INT TERM

rm -rf -- "$lab_root"
mkdir -p -- "$evidence"
printf '%s  %s\n' "$release_sha256" "$release_archive" | sha256sum -c -
printf '%s  %s\n' "$rotation_sha256" "$rotation_archive" | sha256sum -c -
tar -xzf "$release_archive" -C "$lab_root"
tar -xzf "$rotation_archive" -C "$lab_root"
chmod 0755 "$bundle/vpskit" "$bundle/sing-box" "$bundle/lego" "$rotation_bundle/vpskit" "$rotation_bundle/sing-box" "$rotation_bundle/lego"

sha256sum \
  /usr/local/lib/vpskit/bin/sing-box \
  /etc/vpskit/generated/sing-box.json \
  /var/lib/vpskit/certificates/hysteria2.crt \
  /var/lib/vpskit/certificates/hysteria2.key \
  /etc/vpskit/exports/* >"$evidence/before.sha256"

"$bundle/vpskit" bundle verify --dir "$bundle" | tee "$evidence/current-key-verify.json"
"$bundle/vpskit" bundle verify --dir "$rotation_bundle" | tee "$evidence/next-key-verify.json"

cp -a -- "$bundle" "$tampered_bundle"
printf '\n' >>"$tampered_bundle/versions.lock"
set +e
"$bundle/vpskit" bundle verify --dir "$tampered_bundle" >"$evidence/tampered.stdout" 2>"$evidence/tampered.stderr"
tampered_exit=$?
set -e
test "$tampered_exit" -ne 0
grep -Eq 'asset size mismatch|asset digest mismatch' "$evidence/tampered.stderr"
rm -rf -- "$tampered_bundle"

"$bundle/vpskit" update self --bundle-dir "$bundle" | tee "$evidence/update-self.json"
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.21'
test "$(stat -c '%a %U:%G' /etc/vpskit/versions.lock)" = '644 root:root'
python3 - <<'PY'
import hashlib, json
from pathlib import Path

lock = json.load(open('/etc/vpskit/versions.lock'))
assert lock['schema_version'] == 1, lock
assert lock['template_revision'] == 1, lock
assets = {item['id']: item for item in lock['assets']}
assert set(assets) == {'sing-box', 'lego'}, assets
for asset_id, installed in {
    'sing-box': '/usr/local/lib/vpskit/bin/sing-box',
    'lego': '/usr/local/lib/vpskit/bin/lego',
}.items():
    digest = hashlib.sha256(Path(installed).read_bytes()).hexdigest()
    assert digest == assets[asset_id]['binary_sha256'], (asset_id, digest)
    assert assets[asset_id]['source_archive']['sha256'] != digest, asset_id
assert assets['sing-box']['source_commit'] == '25a600db24f7680ad9806ce5427bd0ab8afe1114'
assert assets['lego']['source_commit'] == '3d5a6695e027d625bd34334d516d77f578d43f11'
ownership = json.load(open('/var/lib/vpskit/ownership.json'))
assert '/etc/vpskit/versions.lock' in ownership['files'], ownership['files']
manifest = json.load(open('/root/vpskit-lab21-supply-chain/v0.1.0-lab.21/release-manifest.json'))
assert manifest['schema_version'] == 2, manifest
assert manifest['signing_key_id'].startswith('ed25519-sha256:'), manifest
PY

/usr/local/bin/vpskit bundle verify --dir "$rotation_bundle" | tee "$evidence/installed-next-key-verify.json"
/usr/local/bin/vpskit update core --bundle-dir "$bundle" --channel stable | tee "$evidence/core-current.json"
grep -q 'ALREADY_CURRENT' "$evidence/core-current.json"

/usr/local/bin/vpskit orphan scan | tee "$evidence/orphan-clean.json"
python3 - "$evidence/orphan-clean.json" <<'PY'
import json, sys
value=json.load(open(sys.argv[1]))
assert value['status'] == 'PASS', value
assert value['detail']['mode'] == 'READ_ONLY', value
assert value['detail']['needs_review'] is False, value
assert value['detail']['missing_owned'] == [], value
assert value['detail']['orphan_candidates'] == [], value
PY

printf 'controlled orphan fixture\n' >"$orphan_fixture"
/usr/local/bin/vpskit orphan scan | tee "$evidence/orphan-positive.json"
python3 - "$evidence/orphan-positive.json" "$orphan_fixture" <<'PY'
import json, os, sys
value=json.load(open(sys.argv[1]))
fixture=sys.argv[2]
assert value['detail']['needs_review'] is True, value
assert any(item['path'] == fixture for item in value['detail']['orphan_candidates']), value
assert os.path.exists(fixture), 'read-only scan deleted the fixture'
PY
rm -f -- "$orphan_fixture"
/usr/local/bin/vpskit orphan scan | tee "$evidence/orphan-clean-after.json"
grep -q '"needs_review": false' "$evidence/orphan-clean-after.json"

/usr/local/bin/vpskit status | tee "$evidence/status.json"
/usr/local/bin/vpskit doctor | tee "$evidence/doctor.json"
sha256sum \
  /usr/local/lib/vpskit/bin/sing-box \
  /etc/vpskit/generated/sing-box.json \
  /var/lib/vpskit/certificates/hysteria2.crt \
  /var/lib/vpskit/certificates/hysteria2.key \
  /etc/vpskit/exports/* >"$evidence/after.sha256"
diff -u "$evidence/before.sha256" "$evidence/after.sha256"

rm -f -- "$release_archive" "$rotation_archive"
rm -rf -- "$bundle" "$rotation_bundle"
printf 'LAB21_SUPPLY_CHAIN=PASS\n'
printf 'VERSIONS_LOCK=PASS\n'
printf 'CURRENT_AND_NEXT_KEYS=PASS\n'
printf 'TAMPER_REJECTION=PASS\n'
printf 'ORPHAN_SCAN_READ_ONLY=PASS\n'
