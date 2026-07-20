#!/usr/bin/env bash
set -Eeuo pipefail

run_root='/root/vpskit-lab30-run'
recovery_root='/root/vpskit-lab30-recovery'
lab29_bundle="$run_root/lab29/v0.1.0-lab.29"

test -x "$lab29_bundle/vpskit"
test -f "$recovery_root/backup-id"
backup_id="$(tr -d '\r\n' <"$recovery_root/backup-id")"
test -n "$backup_id"
test -d "$recovery_root/$backup_id"
test -f "$recovery_root/$backup_id/state.json"
test -f "$recovery_root/cloudflare.env"
test -f "$recovery_root/acme.env"

if pgrep -f '^/root/vpskit-lab30-run/lab30/v0\.1\.0-lab\.30/(vpskit|lego) ' >/dev/null; then
    printf 'refusing recovery while lab30 mutation processes still exist\n' >&2
    exit 1
fi

mapfile -t settings < <(python3 - "$recovery_root/$backup_id/state.json" <<'PY'
import json
import sys
from pathlib import Path
state = json.loads(Path(sys.argv[1]).read_text(encoding='utf-8'))
print(state['connect_host'])
print(state['reality_server_name'])
print(state['reality']['listen_port'])
PY
)
connect_host="${settings[0]}"
reality_target="${settings[1]}"
tcp_port="${settings[2]}"

for unit in vpskit-certificate-renew.timer vpskit-certificate-renew.service vpskit-xray.service vpskit-sing-box.service; do
    systemctl disable --now "$unit" >/dev/null 2>&1 || true
done
for path in \
    /etc/systemd/system/vpskit-certificate-renew.timer \
    /etc/systemd/system/vpskit-certificate-renew.service \
    /etc/systemd/system/vpskit-xray.service \
    /etc/systemd/system/vpskit-sing-box.service \
    /usr/local/bin/vpskit; do
    rm -f -- "$path"
done
for path in /etc/vpskit /usr/local/lib/vpskit /var/lib/vpskit /var/log/vpskit; do
    case "$path" in
        /etc/vpskit|/usr/local/lib/vpskit|/var/lib/vpskit|/var/log/vpskit) rm -rf -- "$path" ;;
        *) exit 1 ;;
    esac
done
systemctl daemon-reload
userdel vpskit >/dev/null 2>&1 || true
groupdel vpskit >/dev/null 2>&1 || true

"$lab29_bundle/vpskit" install reality-only \
    --bundle-dir "$lab29_bundle" \
    --connect-host "$connect_host" \
    --reality-server-name "$reality_target" \
    --tcp-port "$tcp_port" >/dev/null

install -d -m 0700 /var/lib/vpskit/backups
cp -a -- "$recovery_root/$backup_id" /var/lib/vpskit/backups/
/usr/local/bin/vpskit restore "$backup_id" --yes >/dev/null
install -o root -g root -m 0600 "$recovery_root/cloudflare.env" /var/lib/vpskit/secrets/cloudflare.env
install -o root -g root -m 0600 "$recovery_root/acme.env" /var/lib/vpskit/secrets/acme.env
systemctl enable --now vpskit-certificate-renew.timer >/dev/null

test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.29'
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-certificate-renew.timer
ss -ltnH | grep -q ':443 '
ss -lunH | grep -q ':443 '
/usr/local/bin/vpskit doctor >/dev/null
/usr/local/bin/vpskit cert status >/dev/null
printf 'LAB29_INDEPENDENT_RECOVERY=PASS\n'
