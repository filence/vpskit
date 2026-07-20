#!/usr/bin/env bash
set -Eeuo pipefail

new_target='www.amazon.com'
service='vpskit-sing-box.service'
sing_box='/usr/local/lib/vpskit/bin/sing-box'
config='/etc/vpskit/generated/sing-box.json'
state='/var/lib/vpskit/state.json'
export_root='/etc/vpskit/exports'
transaction_root='/var/lib/vpskit/transactions'
transaction_id="TX-$(date -u +%Y%m%d-%H%M%S)-reality-target"
transaction_dir="$transaction_root/$transaction_id"
staging="$transaction_dir/staging"
backup="$transaction_dir/backup"

install -d -m 0700 "$transaction_dir" "$staging" "$backup"
cp -a -- "$config" "$backup/sing-box.json"
cp -a -- "$state" "$backup/state.json"
cp -a -- "$export_root/sing-box-reality.json" "$backup/sing-box-reality.json"
cp -a -- "$export_root/mihomo.yaml" "$backup/mihomo.yaml"
cp -a -- "$export_root/share-links.txt" "$backup/share-links.txt"

rollback() {
    local status=$?
    trap - ERR
    install -m 0640 -o root -g vpskit "$backup/sing-box.json" "$config"
    install -m 0600 -o root -g root "$backup/state.json" "$state"
    install -m 0600 -o root -g root "$backup/sing-box-reality.json" "$export_root/sing-box-reality.json"
    install -m 0600 -o root -g root "$backup/mihomo.yaml" "$export_root/mihomo.yaml"
    install -m 0600 -o root -g root "$backup/share-links.txt" "$export_root/share-links.txt"
    systemctl restart "$service" || true
    python3 - "$transaction_dir/transaction.json" "$transaction_id" <<'PY'
import datetime
import json
import pathlib
import sys

path, transaction_id = sys.argv[1:]
record = {
    'schema_version': 1,
    'transaction_id': transaction_id,
    'status': 'ROLLED_BACK',
    'failed_at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'command': 'repair reality target',
}
pathlib.Path(path).write_text(json.dumps(record, indent=2) + '\n', encoding='utf-8')
pathlib.Path(path).chmod(0o600)
PY
    rm -rf -- "$staging"
    echo 'REALITY_TARGET_REPAIR=ROLLED_BACK' >&2
    exit "$status"
}
trap rollback ERR

python3 - "$config" "$state" "$export_root" "$staging" "$new_target" "$transaction_id" <<'PY'
import hashlib
import json
import pathlib
import re
import sys
import urllib.parse

config_path = pathlib.Path(sys.argv[1])
state_path = pathlib.Path(sys.argv[2])
export_root = pathlib.Path(sys.argv[3])
staging = pathlib.Path(sys.argv[4])
new_target = sys.argv[5]
transaction_id = sys.argv[6]

config = json.loads(config_path.read_text(encoding='utf-8'))
vless = [item for item in config['inbounds'] if item.get('type') == 'vless']
if len(vless) != 1:
    raise RuntimeError('expected exactly one managed VLESS inbound')
vless[0]['tls']['server_name'] = new_target
vless[0]['tls']['reality']['handshake']['server'] = new_target
config_bytes = (json.dumps(config, indent=2) + '\n').encode('utf-8')
(staging / 'sing-box.json').write_bytes(config_bytes)

state = json.loads(state_path.read_text(encoding='utf-8'))
state['transaction_id'] = transaction_id
state['reality_server_name'] = new_target
state['config_sha256'] = hashlib.sha256(config_bytes).hexdigest()
(staging / 'state.json').write_text(json.dumps(state, indent=2) + '\n', encoding='utf-8')

client_path = export_root / 'sing-box-reality.json'
client = json.loads(client_path.read_text(encoding='utf-8'))
client_vless = [item for item in client['outbounds'] if item.get('type') == 'vless']
if len(client_vless) != 1:
    raise RuntimeError('expected exactly one VLESS client outbound')
client_vless[0]['tls']['server_name'] = new_target
(staging / 'sing-box-reality.json').write_text(json.dumps(client, indent=2) + '\n', encoding='utf-8')

mihomo = (export_root / 'mihomo.yaml').read_text(encoding='utf-8')
pattern = r'(?m)^(\s*servername:\s*).+$'
if len(re.findall(pattern, mihomo)) != 1:
    raise RuntimeError('expected exactly one Mihomo servername field')
mihomo = re.sub(pattern, lambda match: match.group(1) + json.dumps(new_target), mihomo, count=1)
(staging / 'mihomo.yaml').write_text(mihomo, encoding='utf-8')

links = (export_root / 'share-links.txt').read_text(encoding='utf-8').splitlines()
vless_indexes = [index for index, line in enumerate(links) if line.startswith('vless://')]
if len(vless_indexes) != 1:
    raise RuntimeError('expected exactly one VLESS share link')
index = vless_indexes[0]
parsed = urllib.parse.urlsplit(links[index])
query = urllib.parse.parse_qsl(parsed.query, keep_blank_values=True)
query = [(key, new_target if key == 'sni' else value) for key, value in query]
if sum(1 for key, _ in query if key == 'sni') != 1:
    raise RuntimeError('expected exactly one VLESS sni query field')
links[index] = urllib.parse.urlunsplit((parsed.scheme, parsed.netloc, parsed.path, urllib.parse.urlencode(query), parsed.fragment))
(staging / 'share-links.txt').write_text('\n'.join(links) + '\n', encoding='utf-8')

for path in staging.iterdir():
    path.chmod(0o600)
PY

"$sing_box" check -c "$staging/sing-box.json"

activate_file() {
    local source="$1"
    local destination="$2"
    local mode="$3"
    local owner="$4"
    local group="$5"
    local temporary="${destination}.vpskit-repair.$$"
    install -m "$mode" -o "$owner" -g "$group" "$source" "$temporary"
    mv -f -- "$temporary" "$destination"
}

activate_file "$staging/sing-box.json" "$config" 0640 root vpskit
activate_file "$staging/state.json" "$state" 0600 root root
activate_file "$staging/sing-box-reality.json" "$export_root/sing-box-reality.json" 0600 root root
activate_file "$staging/mihomo.yaml" "$export_root/mihomo.yaml" 0600 root root
activate_file "$staging/share-links.txt" "$export_root/share-links.txt" 0600 root root

systemctl restart "$service"
sleep 2
systemctl is-active --quiet "$service"
ss -ltnH | grep -q ':443 '
ss -lunH | grep -q ':443 '
"$sing_box" check -c "$config"

python3 - "$transaction_dir/transaction.json" "$transaction_id" "$state" "$new_target" <<'PY'
import datetime
import hashlib
import json
import pathlib
import sys

record_path, transaction_id, state_path, new_target = sys.argv[1:]
state_bytes = pathlib.Path(state_path).read_bytes()
record = {
    'schema_version': 1,
    'transaction_id': transaction_id,
    'status': 'COMMITTED',
    'committed_at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'command': 'repair reality target',
    'new_target': new_target,
    'state_sha256': hashlib.sha256(state_bytes).hexdigest(),
}
path = pathlib.Path(record_path)
path.write_text(json.dumps(record, indent=2) + '\n', encoding='utf-8')
path.chmod(0o600)

audit_path = pathlib.Path('/var/log/vpskit/audit.jsonl')
audit = {
    'time': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'transaction_id': transaction_id,
    'command': 'repair reality target',
    'status': 'COMMITTED',
}
with audit_path.open('a', encoding='utf-8') as stream:
    stream.write(json.dumps(audit, separators=(',', ':')) + '\n')
PY

rm -rf -- "$staging"
trap - ERR
echo "REALITY_TARGET_REPAIR=PASS transaction_id=$transaction_id target=$new_target"
