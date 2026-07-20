#!/usr/bin/env bash
set -euo pipefail

python3 - <<'PY'
import hashlib
import json
import pathlib
import re
import subprocess

config_path = pathlib.Path('/etc/vpskit/generated/sing-box.json')
state_path = pathlib.Path('/var/lib/vpskit/state.json')
secret_path = pathlib.Path('/var/lib/vpskit/secrets/instances.json')

config = json.loads(config_path.read_text(encoding='utf-8'))
state = json.loads(state_path.read_text(encoding='utf-8'))
secrets = json.loads(secret_path.read_text(encoding='utf-8'))

reality_inbounds = [item for item in config['inbounds'] if item.get('type') == 'vless']
if len(reality_inbounds) != 1:
    raise SystemExit('REALITY_RUNTIME_CHAIN=FAIL reason=vless_inbound_count')
inbound = reality_inbounds[0]
reality = inbound['tls']['reality']

checks = {
    'config_hash_matches_state': hashlib.sha256(config_path.read_bytes()).hexdigest() == state['config_sha256'],
    'private_key_config_matches_secret': reality['private_key'] == secrets['reality_private_key'],
    'short_id_config_matches_state': reality['short_id'] == [state['reality']['short_id']],
    'uuid_config_matches_secret': inbound['users'][0]['uuid'] == secrets['reality_uuid'],
    'server_name_consistent': (
        inbound['tls']['server_name'] == state['reality_server_name']
        and reality['handshake']['server'] == state['reality_server_name']
    ),
    'private_key_shape': bool(re.fullmatch(r'[A-Za-z0-9_-]{43}', reality['private_key'])),
    'public_key_shape': bool(re.fullmatch(r'[A-Za-z0-9_-]{43}', state['reality']['public_key'])),
    'short_id_shape': bool(re.fullmatch(r'[0-9a-f]{16}', state['reality']['short_id'])),
}

unit = subprocess.run(
    ['systemctl', 'show', 'vpskit-sing-box.service', '--property=ExecStart', '--value'],
    check=True,
    capture_output=True,
    text=True,
).stdout
checks['service_reads_managed_config'] = str(config_path) in unit

for name, passed in checks.items():
    print(f'{name}={str(passed).lower()}')
print('REALITY_RUNTIME_CHAIN=' + ('PASS' if all(checks.values()) else 'FAIL'))
PY

/root/vpskit-v0.1.0-lab.5-doctor doctor | grep -E '"(status|reality_keypair_matches)"'
