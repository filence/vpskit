#!/usr/bin/env bash
set -Eeuo pipefail

archive='/root/v0.1.0-lab.29-linux-amd64.tar.gz'
archive_sha256='5db63c529adde6409ab7a5f63a15a7eca579ddc52d07d0e0b367889cc1bde0cd'
test_root='/root/vpskit-lab29-reality-clean-test'
bundle_parent="$test_root/bundle"
bundle="$bundle_parent/v0.1.0-lab.29"
preserve="$test_root/preserve"
backup_id=''
connect_host=''
reality_target=''
success=false
client_pid=''

cleanup_client() {
    if [ -n "$client_pid" ] && kill -0 "$client_pid" 2>/dev/null; then
        kill "$client_pid" 2>/dev/null || true
        wait "$client_pid" 2>/dev/null || true
    fi
    client_pid=''
}

restore_balanced_after_failure() {
    trap - EXIT HUP INT TERM
    set +e
    cleanup_client
    if [ "$success" = true ]; then
        rm -f -- "$archive"
        case "$test_root" in
            /root/vpskit-lab29-reality-clean-test) rm -rf -- "$test_root" ;;
        esac
        return
    fi
    printf 'REALITY_CLEAN_TEST_RECOVERY=START\n' >&2
    if [ ! -f /var/lib/vpskit/state.json ] && [ -x "$bundle/vpskit" ] && [ -n "$connect_host" ] && [ -n "$reality_target" ]; then
        "$bundle/vpskit" install reality-only --bundle-dir "$bundle" --connect-host "$connect_host" --reality-server-name "$reality_target" --tcp-port 443 >/dev/null 2>&1
    fi
    if [ -n "$backup_id" ] && [ -d "$preserve/$backup_id" ] && [ -x /usr/local/bin/vpskit ]; then
        install -d -m 0700 /var/lib/vpskit/backups
        rm -rf -- "/var/lib/vpskit/backups/$backup_id"
        cp -a -- "$preserve/$backup_id" /var/lib/vpskit/backups/
        /usr/local/bin/vpskit restore "$backup_id" --yes >/dev/null 2>&1
        if [ -f "$preserve/cloudflare.env" ]; then
            install -o root -g root -m 0600 "$preserve/cloudflare.env" /var/lib/vpskit/secrets/cloudflare.env
        fi
        if [ -f "$preserve/acme.env" ]; then
            install -o root -g root -m 0600 "$preserve/acme.env" /var/lib/vpskit/secrets/acme.env
        fi
    fi
    recovered_profile="$(python3 -c 'import json; print(json.load(open("/var/lib/vpskit/state.json"))["profile"])' 2>/dev/null)"
    if [ "$recovered_profile" = balanced ] && /usr/local/bin/vpskit doctor >/dev/null 2>&1; then
        printf 'REALITY_CLEAN_TEST_RECOVERY=PASS\n' >&2
        rm -f -- "$archive"
        case "$test_root" in
            /root/vpskit-lab29-reality-clean-test) rm -rf -- "$test_root" ;;
        esac
    else
        printf 'REALITY_CLEAN_TEST_RECOVERY=FAIL preserve=%s\n' "$test_root" >&2
    fi
}
trap restore_balanced_after_failure EXIT HUP INT TERM

run_client() {
    protocol="$1"
    source_path="$2"
    outbound_type="$3"
    socks_port="$4"
    config_path="$test_root/$protocol.json"
    log_path="$test_root/$protocol.log"
    python3 - "$source_path" "$config_path" "$outbound_type" "$socks_port" <<'PY'
import json
import sys
from pathlib import Path

source, destination, outbound_type, socks_port = sys.argv[1:]
configuration = json.loads(Path(source).read_text(encoding='utf-8'))
configuration['log']['level'] = 'error'
configuration['inbounds'][0]['listen_port'] = int(socks_port)
outbound = next(item for item in configuration['outbounds'] if item['type'] == outbound_type)
outbound['server'] = '127.0.0.1'
Path(destination).write_text(json.dumps(configuration, indent=2) + '\n', encoding='utf-8')
PY
    chmod 0600 "$config_path"
    /usr/local/lib/vpskit/bin/sing-box check -c "$config_path"
    /usr/local/lib/vpskit/bin/sing-box run -c "$config_path" >"$log_path" 2>&1 &
    client_pid=$!
    for _ in $(seq 1 50); do
        if ss -ltnH | grep -q ":$socks_port "; then
            break
        fi
        kill -0 "$client_pid" 2>/dev/null
        sleep 0.1
    done
    ss -ltnH | grep -q ":$socks_port "
    curl --silent --show-error --fail --max-time 20 --socks5-hostname "127.0.0.1:$socks_port" https://api.ipify.org >"$test_root/$protocol.ip"
    grep -Eq '^[0-9a-fA-F:.]+$' "$test_root/$protocol.ip"
    cleanup_client
    printf 'CLEAN_LOOPBACK_%s=PASS\n' "$protocol"
}

assert_reality_only_install() {
    python3 - <<'PY'
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
assert state['schema_version'] == 3
assert state['vpskit_version'] == 'v0.1.0-lab.29'
assert state['profile'] == 'reality-only'
assert state['reality']['enabled'] is True
assert state['reality']['listen_port'] == 443
assert not state['hysteria2'].get('id')
assert state['hysteria2'].get('enabled', False) is False
assert Path('/etc/vpskit/exports/sing-box-reality.json').is_file()
assert not Path('/etc/vpskit/exports/sing-box-hysteria2.json').exists()
assert not Path('/var/lib/vpskit/certificates').exists()
assert not Path('/var/lib/vpskit/secrets/cloudflare.env').exists()
assert not Path('/var/lib/vpskit/secrets/acme.env').exists()
assert not Path('/etc/systemd/system/vpskit-certificate-renew.service').exists()
assert not Path('/etc/systemd/system/vpskit-certificate-renew.timer').exists()
print('REALITY_ONLY_FILES=PASS')
PY
    /usr/local/bin/vpskit doctor >/dev/null
    systemctl is-active --quiet vpskit-sing-box.service
    ! systemctl is-active --quiet vpskit-certificate-renew.timer
    run_client reality /etc/vpskit/exports/sing-box-reality.json vless 12080
    /usr/local/bin/vpskit export --format qr >"$test_root/reality-only-qr.txt"
    grep -Fq 'VPSKit REALITY QR' "$test_root/reality-only-qr.txt"
    ! grep -Fq 'VPSKit HYSTERIA2 QR' "$test_root/reality-only-qr.txt"
    ! grep -Eq 'vless://|hysteria2://' "$test_root/reality-only-qr.txt"
    set +e
    certificate_error="$(/usr/local/bin/vpskit cert status 2>&1)"
    certificate_code=$?
    set -e
    test "$certificate_code" -ne 0
    grep -Fq 'no Hysteria2 instance exists' <<<"$certificate_error"
    printf 'REALITY_ONLY_RUNTIME=PASS\n'
}

test -f "$archive"
case "$test_root" in
    /root/vpskit-lab29-reality-clean-test) rm -rf -- "$test_root" ;;
    *) exit 1 ;;
esac
printf '%s  %s\n' "$archive_sha256" "$archive" | sha256sum -c --strict
install -d -m 0700 "$bundle_parent" "$preserve"
tar -xzf "$archive" -C "$bundle_parent"
chmod 0700 "$bundle"
chmod 0755 "$bundle/vpskit" "$bundle/sing-box" "$bundle/lego"
chmod 0644 "$bundle/release-manifest.json" "$bundle/release-manifest.sig" "$bundle/versions.lock"
"$bundle/vpskit" bundle verify --dir "$bundle"

connect_host="$(python3 -c 'import json; print(json.load(open("/var/lib/vpskit/state.json"))["connect_host"])')"
reality_target="$(python3 -c 'import json; print(json.load(open("/var/lib/vpskit/state.json"))["reality_server_name"])')"
backup_output="$(/usr/local/bin/vpskit backup)"
backup_id="$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["detail"]["backup_id"])' "$backup_output")"
test -d "/var/lib/vpskit/backups/$backup_id"
cp -a -- "/var/lib/vpskit/backups/$backup_id" "$preserve/"
if [ -f /var/lib/vpskit/secrets/cloudflare.env ]; then
    install -o root -g root -m 0600 /var/lib/vpskit/secrets/cloudflare.env "$preserve/cloudflare.env"
fi
if [ -f /var/lib/vpskit/secrets/acme.env ]; then
    install -o root -g root -m 0600 /var/lib/vpskit/secrets/acme.env "$preserve/acme.env"
fi

/usr/local/bin/vpskit uninstall --yes
test ! -e /var/lib/vpskit/state.json
test ! -e /usr/local/bin/vpskit
printf 'BALANCED_UNINSTALL_FOR_CLEAN_TEST=PASS\n'

"$bundle/vpskit" install reality-only --bundle-dir "$bundle" --connect-host "$connect_host" --reality-server-name "$reality_target" --tcp-port 443
assert_reality_only_install
printf 'REALITY_ONLY_INITIAL_INSTALL=PASS\n'

/usr/local/bin/vpskit uninstall --yes
test ! -e /var/lib/vpskit/state.json
test ! -e /usr/local/bin/vpskit
printf 'REALITY_ONLY_UNINSTALL=PASS\n'

"$bundle/vpskit" install reality-only --bundle-dir "$bundle" --connect-host "$connect_host" --reality-server-name "$reality_target" --tcp-port 443 >/dev/null
install -d -m 0700 /var/lib/vpskit/backups
cp -a -- "$preserve/$backup_id" /var/lib/vpskit/backups/
/usr/local/bin/vpskit restore "$backup_id" --yes
if [ -f "$preserve/cloudflare.env" ]; then
    install -o root -g root -m 0600 "$preserve/cloudflare.env" /var/lib/vpskit/secrets/cloudflare.env
fi
if [ -f "$preserve/acme.env" ]; then
    install -o root -g root -m 0600 "$preserve/acme.env" /var/lib/vpskit/secrets/acme.env
fi

test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.29'
/usr/local/bin/vpskit doctor >/dev/null
/usr/local/bin/vpskit cert status >/dev/null
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-certificate-renew.timer
run_client reality /etc/vpskit/exports/sing-box-reality.json vless 12080
run_client hysteria2 /etc/vpskit/exports/sing-box-hysteria2.json hysteria2 12081
orphan_output="$(/usr/local/bin/vpskit orphan scan)"
python3 - "$orphan_output" <<'PY'
import json
import sys

result = json.loads(sys.argv[1])
assert result['status'] == 'PASS', result
assert (result.get('detail') or {}).get('needs_review') in (False, None), result
PY

success=true
printf 'LAB29_REALITY_CLEAN_CYCLE=PASS\n'
