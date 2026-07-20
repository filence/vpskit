#!/usr/bin/env bash
set -eu

python3 - <<'PY'
import json
from pathlib import Path

server = json.loads(Path('/etc/vpskit/generated/sing-box.json').read_text())
reality = json.loads(Path('/etc/vpskit/exports/sing-box-reality.json').read_text())
hy2 = json.loads(Path('/etc/vpskit/exports/sing-box-hysteria2.json').read_text())
secrets = json.loads(Path('/var/lib/vpskit/secrets/instances.json').read_text())
state = json.loads(Path('/var/lib/vpskit/state.json').read_text())

server_reality = next(item for item in server['inbounds'] if item['type'] == 'vless')
server_hy2 = next(item for item in server['inbounds'] if item['type'] == 'hysteria2')
client_reality = next(item for item in reality['outbounds'] if item['type'] == 'vless')
client_hy2 = next(item for item in hy2['outbounds'] if item['type'] == 'hysteria2')

checks = {
    'reality_uuid': server_reality['users'][0]['uuid'] == client_reality['uuid'] == secrets['reality_uuid'],
    'reality_flow': server_reality['users'][0]['flow'] == client_reality['flow'],
    'reality_private': server_reality['tls']['reality']['private_key'] == secrets['reality_private_key'],
    'reality_public': client_reality['tls']['reality']['public_key'] == state['reality']['public_key'],
    'reality_short_id': server_reality['tls']['reality']['short_id'][0] == client_reality['tls']['reality']['short_id'] == state['reality']['short_id'],
    'reality_sni': server_reality['tls']['server_name'] == client_reality['tls']['server_name'],
    'reality_port': server_reality['listen_port'] == client_reality['server_port'],
    'hy2_password': server_hy2['users'][0]['password'] == client_hy2['password'] == secrets['hysteria2_password'],
    'hy2_sni': server_hy2['tls']['server_name'] == client_hy2['tls']['server_name'],
    'hy2_port': server_hy2['listen_port'] == client_hy2['server_port'],
}
failed = [name for name, passed in checks.items() if not passed]
if failed:
    raise SystemExit('CLIENT_CONSISTENCY=FAIL checks=' + ','.join(failed))
print('CLIENT_CONSISTENCY=PASS checks=' + str(len(checks)))
PY
