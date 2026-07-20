#!/usr/bin/env bash
set -Eeuo pipefail

upload='/root/vpskit-xray-compat-xray'
expected_sha256='8255dd939c34cf966cc91517b6324dd3c8d0bcf49ffac8beca049a38c46845ed'
runtime='/root/vpskit-xray-compat-runtime'
unit='vpskit-xray-compat.service'
legacy_runtime='/run/vpskit-xray-compat'

test -f "$upload"
printf '%s  %s\n' "$expected_sha256" "$upload" | sha256sum -c --strict >/dev/null
systemctl stop "$unit" >/dev/null 2>&1 || true
case "$legacy_runtime" in
  /run/vpskit-xray-compat) rm -rf -- "$legacy_runtime" ;;
  *) printf 'refusing unexpected legacy compatibility runtime path\n' >&2; exit 1 ;;
esac
case "$runtime" in
  /root/vpskit-xray-compat-runtime) rm -rf -- "$runtime" ;;
  *) printf 'refusing unexpected compatibility runtime path\n' >&2; exit 1 ;;
esac
install -d -m 0700 "$runtime"
install -m 0755 "$upload" "$runtime/xray"

python3 - "$runtime/config.json" <<'PY'
import json
import os
import pathlib
import sys

destination = pathlib.Path(sys.argv[1])
state = json.loads(pathlib.Path('/var/lib/vpskit/state.json').read_text(encoding='utf-8'))
secrets = json.loads(pathlib.Path('/var/lib/vpskit/secrets/instances.json').read_text(encoding='utf-8'))
target = state['reality_server_name']
configuration = {
    'log': {'loglevel': 'warning'},
    'inbounds': [{
        'listen': '127.0.0.1',
        'port': 14443,
        'protocol': 'vless',
        'settings': {
            'clients': [{
                'id': secrets['reality_uuid'],
                'flow': 'xtls-rprx-vision',
            }],
            'decryption': 'none',
        },
        'streamSettings': {
            'network': 'raw',
            'security': 'reality',
            'realitySettings': {
                'show': False,
                'target': f'{target}:443',
                'xver': 0,
                'serverNames': [target],
                'privateKey': secrets['reality_private_key'],
                'shortIds': [state['reality']['short_id']],
            },
        },
        'tag': 'reality-compat-in',
    }],
    'outbounds': [{'protocol': 'freedom', 'tag': 'direct'}],
}
destination.write_text(json.dumps(configuration), encoding='utf-8')
os.chmod(destination, 0o600)
PY

"$runtime/xray" run -test -config "$runtime/config.json" >/dev/null
systemd-run \
  --quiet \
  --collect \
  --unit="${unit%.service}" \
  --property='RuntimeMaxSec=180' \
  --property='NoNewPrivileges=yes' \
  "$runtime/xray" run -config "$runtime/config.json"

for _ in $(seq 1 50); do
  if ss -ltnH | grep -q '127.0.0.1:14443 '; then
    printf 'XRAY_COMPAT_SERVER=PASS listen=loopback:14443 lifetime_seconds=180\n'
    exit 0
  fi
  systemctl is-active --quiet "$unit" || break
  sleep 0.1
done
journalctl -u "$unit" -n 20 --no-pager >&2 || true
exit 1
