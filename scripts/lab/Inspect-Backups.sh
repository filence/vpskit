#!/usr/bin/env bash
set -euo pipefail
python3 <<'PY'
import hashlib
import json
import pathlib

root = pathlib.Path('/var/lib/vpskit/backups')
for manifest_path in sorted(root.glob('BK-*/backup.json')):
    manifest = json.loads(manifest_path.read_text(encoding='utf-8'))
    print(manifest_path.parent.name, manifest.get('state_transaction_id'))
    for entry in manifest['entries']:
        if entry['source'] in ('/var/lib/vpskit/state.json', '/etc/vpskit/generated/sing-box.json'):
            path = manifest_path.parent / entry['relative']
            digest = hashlib.sha256(path.read_bytes()).hexdigest()
            print(entry['source'], entry['relative'], entry['sha256'], digest, entry['sha256'] == digest)
            if entry['source'] == '/var/lib/vpskit/state.json':
                state = json.loads(path.read_text(encoding='utf-8'))
                print('state_config_sha256', state.get('config_sha256'))
PY
stat -c '%a %U:%G %n' /etc/vpskit/generated/sing-box.json /var/lib/vpskit/state.json
sha256sum /etc/vpskit/generated/sing-box.json /var/lib/vpskit/state.json
systemctl is-active --quiet vpskit-sing-box.service
ss -ltnH | grep -q ':443 '
ss -lunH | grep -q ':443 '
echo 'BACKUP_INSPECTION=PASS'
