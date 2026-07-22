#!/usr/bin/env bash
set -euo pipefail

state=/var/lib/vpskit/state.json
secrets=/var/lib/vpskit/secrets/instances.json
server=/etc/vpskit/generated/xray.json
singbox_export=/etc/vpskit/exports/sing-box-reality.json
mihomo_export=/etc/vpskit/exports/mihomo.yaml
xray=/usr/local/lib/vpskit/bin/xray

for path in "$state" "$secrets" "$server" "$singbox_export" "$mihomo_export" "$xray"; do
  test -f "$path"
done

python3 - "$state" "$secrets" "$server" "$singbox_export" "$mihomo_export" "$xray" <<'PY'
import json
import re
import subprocess
import sys

state_path, secrets_path, server_path, export_path, mihomo_path, xray_path = sys.argv[1:]

def load(path):
    with open(path, encoding="utf-8") as stream:
        return json.load(stream)

state = load(state_path)
secrets = load(secrets_path)
server = load(server_path)
export = load(export_path)
with open(mihomo_path, encoding="utf-8") as stream:
    mihomo = stream.read()

inbound = next(item for item in server["inbounds"] if item["protocol"] == "vless")
outbound = next(item for item in export["outbounds"] if item["type"] == "vless")
reality = inbound["streamSettings"]["realitySettings"]
server_private = reality["privateKey"]
derived = subprocess.run(
    [xray_path, "x25519", "-i", server_private],
    check=True,
    capture_output=True,
    text=True,
).stdout
derived_public = next(
    line.split(": ", 1)[1].strip()
    for line in derived.splitlines()
    if line.startswith("Password (PublicKey):")
)

state_public = state["reality"]["public_key"]
state_short_id = state["reality"]["short_id"]
assert secrets["reality_private_key"] == server_private
assert state_public == derived_public
assert state_public == outbound["tls"]["reality"]["public_key"]
assert state_short_id == reality["shortIds"][0]
assert state_short_id == outbound["tls"]["reality"]["short_id"]
assert re.search(r"^\s*public-key:\s*['\"]?" + re.escape(state_public) + r"['\"]?\s*$", mihomo, re.MULTILINE)
assert re.search(r"^\s*short-id:\s*['\"]?" + re.escape(state_short_id) + r"['\"]?\s*$", mihomo, re.MULTILINE)

print("REALITY_KEY_CONSISTENCY=PASS")
print("REALITY_SHORT_ID_CONSISTENCY=PASS")
PY
