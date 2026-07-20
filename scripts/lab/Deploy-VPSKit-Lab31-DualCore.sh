#!/usr/bin/env bash
set -Eeuo pipefail

lab31_archive='/root/v0.1.0-lab.31-linux-amd64.tar.gz'
lab31_sha256='2cc3b8e94242053351e9df5e483f37425f9a39f57dacafa22b1a45bb6066ca38'
lab29_archive='/root/v0.1.0-lab.29-linux-amd64.tar.gz'
lab29_sha256='5db63c529adde6409ab7a5f63a15a7eca579ddc52d07d0e0b367889cc1bde0cd'
run_root='/root/vpskit-lab31-run'
recovery_root='/root/vpskit-lab31-recovery'
lab31_bundle="$run_root/lab31/v0.1.0-lab.31"
lab29_bundle="$run_root/lab29/v0.1.0-lab.29"
backup_id=''
success=false
client_pid=''

cleanup_client() {
    if [ -n "$client_pid" ] && kill -0 "$client_pid" 2>/dev/null; then
        kill "$client_pid" 2>/dev/null || true
        wait "$client_pid" 2>/dev/null || true
    fi
    client_pid=''
}

recover_lab29() {
    trap - EXIT HUP INT TERM
    set +e
    cleanup_client
    if [ "$success" = true ]; then
        rm -f -- "$lab31_archive" "$lab29_archive"
        case "$run_root" in
            /root/vpskit-lab31-run) rm -rf -- "$run_root" ;;
        esac
        return
    fi
    printf 'LAB31_RECOVERY=START\n' >&2
    if [ -e /var/lib/vpskit/state.json ] && [ -x /usr/local/bin/vpskit ]; then
        /usr/local/bin/vpskit uninstall --yes >/dev/null 2>&1
    fi
    rm -f -- /etc/systemd/system/vpskit-xray.service
    systemctl daemon-reload >/dev/null 2>&1
    if [ ! -e /var/lib/vpskit/state.json ] && [ -x "$lab29_bundle/vpskit" ]; then
        "$lab29_bundle/vpskit" install reality-only \
            --bundle-dir "$lab29_bundle" \
            --connect-host "$connect_host" \
            --reality-server-name "$reality_target" \
            --tcp-port "$tcp_port" >/dev/null 2>&1
    fi
    if [ -n "$backup_id" ] && [ -d "$recovery_root/$backup_id" ] && [ -x /usr/local/bin/vpskit ]; then
        install -d -m 0700 /var/lib/vpskit/backups
        rm -rf -- "/var/lib/vpskit/backups/$backup_id"
        cp -a -- "$recovery_root/$backup_id" /var/lib/vpskit/backups/
        /usr/local/bin/vpskit restore "$backup_id" --yes >/dev/null 2>&1
        install -o root -g root -m 0600 "$recovery_root/cloudflare.env" /var/lib/vpskit/secrets/cloudflare.env
        install -o root -g root -m 0600 "$recovery_root/acme.env" /var/lib/vpskit/secrets/acme.env
    fi
    if [ "$(/usr/local/bin/vpskit version 2>/dev/null)" = 'vpskit v0.1.0-lab.29' ] && /usr/local/bin/vpskit doctor >/dev/null 2>&1; then
        printf 'LAB31_RECOVERY=PASS\n' >&2
    else
        printf 'LAB31_RECOVERY=FAIL preserved=%s\n' "$recovery_root" >&2
    fi
}
trap recover_lab29 EXIT HUP INT TERM

run_client() {
    local protocol="$1"
    local source_path="$2"
    local outbound_type="$3"
    local socks_port="$4"
    local config_path="$run_root/$protocol-client.json"
    local log_path="$run_root/$protocol-client.log"
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
    curl --silent --show-error --fail --max-time 20 --socks5-hostname "127.0.0.1:$socks_port" https://api.ipify.org >"$run_root/$protocol.ip"
    grep -Eq '^[0-9a-fA-F:.]+$' "$run_root/$protocol.ip"
    cleanup_client
    printf 'LAB31_LOOPBACK_%s=PASS\n' "${protocol^^}"
}

test -f "$lab31_archive"
test -f "$lab29_archive"
printf '%s  %s\n' "$lab31_sha256" "$lab31_archive" | sha256sum -c --strict
printf '%s  %s\n' "$lab29_sha256" "$lab29_archive" | sha256sum -c --strict

case "$run_root" in
    /root/vpskit-lab31-run) rm -rf -- "$run_root" ;;
    *) exit 1 ;;
esac
case "$recovery_root" in
    /root/vpskit-lab31-recovery) rm -rf -- "$recovery_root" ;;
    *) exit 1 ;;
esac
install -d -m 0700 "$run_root/lab31" "$run_root/lab29" "$recovery_root"
tar -xzf "$lab31_archive" -C "$run_root/lab31"
tar -xzf "$lab29_archive" -C "$run_root/lab29"
chmod 0700 "$lab31_bundle" "$lab29_bundle"
chmod 0755 "$lab31_bundle/vpskit" "$lab31_bundle/sing-box" "$lab31_bundle/xray" "$lab31_bundle/lego"
chmod 0755 "$lab29_bundle/vpskit" "$lab29_bundle/sing-box" "$lab29_bundle/lego"
chmod 0644 "$lab31_bundle/release-manifest.json" "$lab31_bundle/release-manifest.sig" "$lab31_bundle/versions.lock"
chmod 0644 "$lab29_bundle/release-manifest.json" "$lab29_bundle/release-manifest.sig" "$lab29_bundle/versions.lock"
"$lab31_bundle/vpskit" bundle verify --dir "$lab31_bundle"

mapfile -t settings < <(python3 - <<'PY'
import json
from pathlib import Path
state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
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

backup_output="$(/usr/local/bin/vpskit backup)"
backup_id="$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["detail"]["backup_id"])' "$backup_output")"
test -d "/var/lib/vpskit/backups/$backup_id"
cp -a -- "/var/lib/vpskit/backups/$backup_id" "$recovery_root/"
install -o root -g root -m 0600 /var/lib/vpskit/secrets/cloudflare.env "$recovery_root/cloudflare.env"
install -o root -g root -m 0600 /var/lib/vpskit/secrets/acme.env "$recovery_root/acme.env"
printf '%s\n' "$backup_id" >"$recovery_root/backup-id"
chmod 0600 "$recovery_root/backup-id"
printf 'LAB29_RECOVERY_SNAPSHOT=PASS\n'

/usr/local/bin/vpskit uninstall --yes
test ! -e /var/lib/vpskit/state.json
test ! -e /usr/local/bin/vpskit
test ! -e /etc/systemd/system/vpskit-sing-box.service
printf 'LAB29_UNINSTALL=PASS\n'

set -a
# shellcheck source=/dev/null
source "$recovery_root/cloudflare.env"
# shellcheck source=/dev/null
source "$recovery_root/acme.env"
set +a
export VPSKIT_CF_DNS_API_TOKEN="$CF_DNS_API_TOKEN"
export VPSKIT_ACME_SERVER="$ACME_SERVER"
export VPSKIT_ACME_EMAIL="$ACME_EMAIL"
if [ -n "${ACME_EAB_KID:-}" ]; then
    export VPSKIT_ACME_EAB_KID="$ACME_EAB_KID"
    export VPSKIT_ACME_EAB_HMAC="$ACME_EAB_HMAC"
fi

"$lab31_bundle/vpskit" install balanced \
    --bundle-dir "$lab31_bundle" \
    --domain "$domain" \
    --connect-host "$connect_host" \
    --reality-server-name "$reality_target" \
    --tcp-port "$tcp_port" \
    --udp-port "$udp_port" \
    --existing-certificate "$recovery_root/$backup_id/certificates/hysteria2.crt" \
    --existing-key "$recovery_root/$backup_id/certificates/hysteria2.key"
unset VPSKIT_CF_DNS_API_TOKEN CF_DNS_API_TOKEN VPSKIT_ACME_EAB_KID VPSKIT_ACME_EAB_HMAC ACME_EAB_KID ACME_EAB_HMAC

test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.31'
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-certificate-renew.timer
/usr/local/lib/vpskit/bin/sing-box check -c /etc/vpskit/generated/sing-box.json
/usr/local/lib/vpskit/bin/xray run -test -config /etc/vpskit/generated/xray.json
xray_version_output="$(/usr/local/lib/vpskit/bin/xray version)"
grep -Fq 'Xray 26.3.27' <<<"$xray_version_output"

python3 - <<'PY'
import hashlib
import json
from pathlib import Path

state = json.loads(Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
assert state['schema_version'] == 4, state
assert state['vpskit_version'] == 'v0.1.0-lab.31', state
assert state['profile'] == 'balanced', state
assert state['core']['id'] == 'sing-box', state
assert state['core']['version'] == '1.13.14', state
assert state['reality_core']['id'] == 'xray', state
assert state['reality_core']['version'] == '26.3.27', state
assert state['reality']['enabled'] is True, state
assert state['hysteria2']['enabled'] is True, state
sing_box_config = Path('/etc/vpskit/generated/sing-box.json').read_bytes()
xray_config = Path('/etc/vpskit/generated/xray.json').read_bytes()
assert hashlib.sha256(sing_box_config).hexdigest() == state['config_sha256'], state
assert hashlib.sha256(xray_config).hexdigest() == state['reality_config_sha256'], state
sing_box = json.loads(sing_box_config)
xray = json.loads(xray_config)
assert [item['type'] for item in sing_box['inbounds']] == ['hysteria2'], sing_box
assert [item['protocol'] for item in xray['inbounds']] == ['vless'], xray
assert xray['inbounds'][0]['streamSettings']['security'] == 'reality', xray
assert Path('/etc/vpskit/exports/mihomo.yaml').is_file()
assert Path('/etc/vpskit/exports/sing-box-reality.json').is_file()
assert Path('/etc/vpskit/exports/sing-box-hysteria2.json').is_file()
print('LAB31_STATE_AND_CONFIG=PASS')
PY

test "$(systemctl show vpskit-xray.service --property=User --value)" = vpskit
test "$(systemctl show vpskit-sing-box.service --property=User --value)" = vpskit
runuser -u vpskit -- test -r /etc/vpskit/generated/xray.json
runuser -u vpskit -- test -r /etc/vpskit/generated/sing-box.json
ss -ltnp | grep -q ':443 .*xray'
ss -lunp | grep -q ':443 .*sing-box'
/usr/local/bin/vpskit doctor
/usr/local/bin/vpskit cert status >/dev/null
run_client reality /etc/vpskit/exports/sing-box-reality.json vless 12080
run_client hysteria2 /etc/vpskit/exports/sing-box-hysteria2.json hysteria2 12081

orphan_output="$(/usr/local/bin/vpskit orphan scan)"
python3 - "$orphan_output" <<'PY'
import json
import sys
result = json.loads(sys.argv[1])
assert result['status'] == 'PASS', result
assert (result.get('detail') or {}).get('needs_review') in (False, None), result
print('LAB31_ORPHAN_SCAN=PASS')
PY

/usr/local/bin/vpskit export --format qr >"$run_root/qr.txt"
grep -Fq 'VPSKit REALITY QR' "$run_root/qr.txt"
grep -Fq 'VPSKit HYSTERIA2 QR' "$run_root/qr.txt"
if grep -Eq 'vless://|hysteria2://' "$run_root/qr.txt"; then
    exit 1
fi
/usr/local/bin/vpskit status
ps -C xray -C sing-box -o pid=,rss=,%cpu=,cmd=

success=true
printf 'LAB31_DUAL_CORE_DEPLOY=PASS\n'
