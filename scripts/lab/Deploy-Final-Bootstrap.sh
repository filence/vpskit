#!/usr/bin/env bash
set -Eeuo pipefail

readonly recovery_root='/root/vpskit-final-bootstrap-lab33'
readonly snapshot='/root/vpskit-final-bootstrap-lab33-preinstall.tar.gz'
readonly archive='/root/vpskit-final-bootstrap-lab33-release.tar.gz'
readonly installer='/root/vpskit-final-bootstrap-lab33-install.sh'
readonly loopback='/root/vpskit-final-bootstrap-lab33-loopback.sh'
readonly install_log="$recovery_root/bootstrap-install.log"
readonly client_root="$recovery_root/client"

success=false

restore_previous_installation() {
    local original_status="$?"
    trap - EXIT HUP INT TERM
    if [ "$success" = true ]; then
        return 0
    fi

    set +e
    printf 'FINAL_BOOTSTRAP_RECOVERY=START\n' >&2
    systemctl disable --now vpskit-xray.service vpskit-sing-box.service vpskit-certificate-renew.timer >/dev/null 2>&1
    if [ -x /usr/local/bin/vpskit ] && [ -f /var/lib/vpskit/ownership.json ]; then
        /usr/local/bin/vpskit uninstall --yes >/dev/null 2>&1
    fi
    userdel vpskit >/dev/null 2>&1
    groupdel vpskit >/dev/null 2>&1

    old_group="$(cut -d: -f1 "$recovery_root/group-entry")"
    old_gid="$(cut -d: -f3 "$recovery_root/group-entry")"
    old_user="$(cut -d: -f1 "$recovery_root/passwd-entry")"
    old_uid="$(cut -d: -f3 "$recovery_root/passwd-entry")"
    old_home="$(cut -d: -f6 "$recovery_root/passwd-entry")"
    old_shell="$(cut -d: -f7 "$recovery_root/passwd-entry")"
    groupadd --system --gid "$old_gid" "$old_group" >/dev/null 2>&1
    useradd --system --uid "$old_uid" --gid "$old_group" --home-dir "$old_home" --shell "$old_shell" "$old_user" >/dev/null 2>&1
    tar --numeric-owner -xzf "$snapshot" -C /
    systemctl daemon-reload >/dev/null 2>&1
    systemctl enable --now vpskit-xray.service vpskit-sing-box.service vpskit-certificate-renew.timer >/dev/null 2>&1
    if [ "$(/usr/local/bin/vpskit version 2>/dev/null)" = 'vpskit v0.1.0-lab.33' ] && /usr/local/bin/vpskit doctor >/dev/null 2>&1; then
        printf 'FINAL_BOOTSTRAP_RECOVERY=PASS\n' >&2
    else
        printf 'FINAL_BOOTSTRAP_RECOVERY=FAIL preserved=%s\n' "$recovery_root" >&2
    fi
    exit "$original_status"
}
trap restore_previous_installation EXIT HUP INT TERM

for path in "$recovery_root" "$snapshot" "$archive" "$installer" "$loopback"; do
    test -e "$path"
    test ! -L "$path"
done
test -f "$recovery_root/cloudflare.env"
test -f "$recovery_root/acme.env"
test -f "$recovery_root/hysteria2.crt"
test -f "$recovery_root/hysteria2.key"
tar -tzf "$snapshot" >/dev/null
bash "$installer" --archive "$archive" --verify-only >/dev/null
printf 'FINAL_BOOTSTRAP_ARCHIVE_VERIFY=PASS\n'

mapfile -t settings < <(python3 - "$recovery_root" <<'PY'
import json
import sys
from pathlib import Path

root = Path(sys.argv[1])
backup_id = (root / 'backup-id').read_text(encoding='utf-8').strip()
state = json.loads((root / backup_id / 'state.json').read_text(encoding='utf-8'))
print(state['connect_host'])
print(state['domain'])
print(state['reality_server_name'])
print(state['reality']['listen_port'])
print(state['hysteria2']['listen_port'])
PY
)
connect_host="${settings[0]}"
domain="${settings[1]}"
reality_target="${settings[2]}"
tcp_port="${settings[3]}"
udp_port="${settings[4]}"

set -a
# shellcheck source=/dev/null
source "$recovery_root/cloudflare.env"
# shellcheck source=/dev/null
source "$recovery_root/acme.env"
set +a
test -n "${CF_DNS_API_TOKEN:-}"
test -n "${ACME_EMAIL:-}"
case "${ACME_SERVER:-}" in
    zerossl|*zerossl*) ca_choice='1' ;;
    letsencrypt|*letsencrypt*) ca_choice='2' ;;
    *) printf 'FINAL_BOOTSTRAP_INSTALL=FAIL reason=unsupported_acme_server\n' >&2; exit 1 ;;
esac
if [ -n "${ACME_EAB_KID:-}" ] || [ -n "${ACME_EAB_HMAC:-}" ]; then
    test -n "${ACME_EAB_KID:-}"
    test -n "${ACME_EAB_HMAC:-}"
    export VPSKIT_ACME_EAB_KID="$ACME_EAB_KID"
    export VPSKIT_ACME_EAB_HMAC="$ACME_EAB_HMAC"
fi

/usr/local/bin/vpskit doctor >/dev/null
/usr/local/bin/vpskit uninstall --yes >"$recovery_root/uninstall.log"
test ! -e /var/lib/vpskit/state.json
test ! -e /usr/local/bin/vpskit
test ! -e /etc/systemd/system/vpskit-sing-box.service
test ! -e /etc/systemd/system/vpskit-xray.service
printf 'FINAL_BOOTSTRAP_UNINSTALL=PASS\n'

: >"$install_log"
chmod 0600 "$install_log"
{
    printf '%s\n' \
        '1' \
        "$reality_target" \
        "$tcp_port" \
        "$domain" \
        "$connect_host" \
        "$udp_port" \
        '2' \
        "$recovery_root/hysteria2.crt" \
        "$recovery_root/hysteria2.key" \
        "$ca_choice" \
        "$ACME_EMAIL" \
        "$CF_DNS_API_TOKEN" \
        'INSTALL'
} | bash "$installer" --archive "$archive" >"$install_log" 2>&1
unset VPSKIT_ACME_EAB_KID VPSKIT_ACME_EAB_HMAC ACME_EAB_KID ACME_EAB_HMAC CF_DNS_API_TOKEN
printf 'FINAL_BOOTSTRAP_INSTALL=PASS\n'

test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.33'
/usr/local/bin/vpskit doctor >/dev/null
/usr/local/bin/vpskit cert status >/dev/null
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-certificate-renew.timer
/usr/local/lib/vpskit/bin/sing-box check -c /etc/vpskit/generated/sing-box.json >/dev/null
/usr/local/lib/vpskit/bin/xray run -test -config /etc/vpskit/generated/xray.json >/dev/null

python3 - "$recovery_root" <<'PY'
import hashlib
import json
import sys
from pathlib import Path

root = Path(sys.argv[1])
backup_id = (root / 'backup-id').read_text(encoding='utf-8').strip()
previous = json.loads((root / backup_id / 'state.json').read_text(encoding='utf-8'))
current = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
assert current['schema_version'] == 5, current
assert current['vpskit_version'] == 'v0.1.0-lab.33', current
assert current['profile'] == 'balanced', current
assert current['config_revision'] == 1, current
assert current['connect_host'] == previous['connect_host'], current
assert current['domain'] == previous['domain'], current
assert current['reality_server_name'] == previous['reality_server_name'], current
assert current['reality']['listen_port'] == previous['reality']['listen_port'], current
assert current['hysteria2']['listen_port'] == previous['hysteria2']['listen_port'], current
assert hashlib.sha256(Path('/var/lib/vpskit/certificates/hysteria2.crt').read_bytes()).hexdigest() == hashlib.sha256((root / 'hysteria2.crt').read_bytes()).hexdigest()
print('FINAL_BOOTSTRAP_STATE=PASS')
PY

bash "$loopback" >"$recovery_root/loopback.log" 2>&1
grep -q '^REMOTE_LOOPBACK_CLIENTS=PASS$' "$recovery_root/loopback.log"
printf 'FINAL_BOOTSTRAP_LOOPBACK=PASS\n'

orphan_output="$(/usr/local/bin/vpskit orphan scan)"
python3 - "$orphan_output" <<'PY'
import json
import sys

result = json.loads(sys.argv[1])
assert result['status'] == 'PASS', result
assert (result.get('detail') or {}).get('needs_review') in (False, None), result
PY
printf 'FINAL_BOOTSTRAP_ORPHAN_SCAN=PASS\n'

install -d -o root -g root -m 0700 "$client_root"
/usr/local/bin/vpskit export --format bundle --output-dir "$client_root" >"$recovery_root/client-export.json"
mapfile -t client_bundles < <(find "$client_root" -maxdepth 1 -type f -name 'vpskit-client-*.zip' -print)
test "${#client_bundles[@]}" -eq 1
install -o root -g root -m 0600 "${client_bundles[0]}" "$recovery_root/new-client.zip"
python3 - "$recovery_root/new-client.zip" <<'PY'
import hashlib
import json
import sys
import zipfile

with zipfile.ZipFile(sys.argv[1]) as archive:
    names = set(archive.namelist())
    required = {'manifest.json', 'mihomo.yaml', 'sing-box-reality.json', 'sing-box-hysteria2.json', 'share-links.txt'}
    assert required <= names, names
    manifest = json.loads(archive.read('manifest.json'))
    assert manifest['schema_version'] == 1, manifest
    assert manifest['profile'] == 'balanced', manifest
    assert manifest['config_revision'] == 1, manifest
    for entry in manifest['files']:
        payload = archive.read(entry['name'])
        assert len(payload) == entry['size'], entry
        assert hashlib.sha256(payload).hexdigest() == entry['sha256'], entry
PY
printf 'FINAL_BOOTSTRAP_CLIENT_BUNDLE=PASS\n'

success=true
printf 'FINAL_BOOTSTRAP_AUTOMATED_ACCEPTANCE=PASS\n'
