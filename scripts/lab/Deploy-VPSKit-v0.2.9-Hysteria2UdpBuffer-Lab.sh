#!/usr/bin/env bash
set -euo pipefail
umask 077

readonly version='v0.2.9-lab.1'
readonly archive="/root/${version}-linux-amd64.tar.gz"
readonly archive_sha256='455a16e2eb36dcb8de452322c79315d33e81c2395ab90af9095a7fa2b4ebb1c6'
readonly root="/root/vpskit-${version}-upgrade"
readonly bundle="${root}/${version}"
readonly backup="/var/lib/vpskit/backups/${version}-cli-upgrade"
readonly backup_binary="${backup}/vpskit-before-${version}"
readonly sysctl_file='/etc/sysctl.d/70-vpskit-hysteria2-udp-buffer.conf'
readonly ownership_file='/var/lib/vpskit/hysteria2/udp-buffer.json'
readonly target_bytes=2097152
readonly baseline_bytes=212992

test -f "$archive"
test -x /usr/local/bin/vpskit
test "$(sha256sum "$archive" | awk '{print $1}')" = "$archive_sha256"
test ! -e "$root"
test ! -e "$backup_binary"

# The prior tuning was created only by this lab, before VPSKit had an ownership
# record.  It is deliberately returned to the recorded Debian baseline so the
# new command can own a complete, reversible transition.
test -f "$sysctl_file"
test ! -e "$ownership_file"
for key in net.core.rmem_default net.core.wmem_default net.core.rmem_max net.core.wmem_max; do
  test "$(sysctl -n "$key")" = "$target_bytes"
done

state_before="$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
readonly state_before
publication_before="$(sha256sum /var/lib/vpskit/subscription-state.json | awk '{print $1}')"
readonly publication_before
install -d -m 0700 "$root"
install -d -m 0700 "$backup"
cp --preserve=mode,timestamps "$sysctl_file" "$root/manual-udp-buffer-before-v0.2.9.conf"
tar -xzf "$archive" -C "$root"
"$bundle/vpskit" bundle verify --dir "$bundle"
install -m 0700 /usr/local/bin/vpskit "$backup_binary"
install -m 0755 "$bundle/vpskit" /usr/local/bin/vpskit
test "$(/usr/local/bin/vpskit version)" = "vpskit ${version}"

rm -f -- "$sysctl_file"
sysctl -w \
  "net.core.rmem_default=$baseline_bytes" \
  "net.core.wmem_default=$baseline_bytes" \
  "net.core.rmem_max=$baseline_bytes" \
  "net.core.wmem_max=$baseline_bytes" >/dev/null
systemctl restart vpskit-sing-box.service
systemctl is-active --quiet vpskit-sing-box.service

/usr/local/bin/vpskit hysteria2 udp-buffer plan --profile conservative-2mib >/root/vpskit-hysteria2-udp-buffer-plan.json
/usr/local/bin/vpskit hysteria2 udp-buffer apply --profile conservative-2mib --yes >/root/vpskit-hysteria2-udp-buffer-apply.json
/usr/local/bin/vpskit hysteria2 udp-buffer status >/root/vpskit-hysteria2-udp-buffer-status.json
python3 - <<'PY'
import json
from pathlib import Path

plan = json.loads(Path('/root/vpskit-hysteria2-udp-buffer-plan.json').read_text(encoding='utf-8'))
applied = json.loads(Path('/root/vpskit-hysteria2-udp-buffer-apply.json').read_text(encoding='utf-8'))
status = json.loads(Path('/root/vpskit-hysteria2-udp-buffer-status.json').read_text(encoding='utf-8'))
assert plan['status'] == 'PASS', plan
assert plan['detail']['target_sysctls']['net.core.rmem_default'] == 2097152, plan
assert applied['status'] == 'PASS' and applied['detail']['result'] == 'APPLIED', applied
assert applied['detail']['socket_buffers']['receive_bytes'] >= 2097152, applied
assert applied['detail']['socket_buffers']['send_bytes'] >= 2097152, applied
assert status['status'] == 'PASS' and status['detail']['managed'] is True, status
PY

# Exercise the rollback path before leaving the final, managed 2 MiB profile
# active.  No node, rule or subscription revision is expected from either step.
/usr/local/bin/vpskit hysteria2 udp-buffer rollback --yes >/root/vpskit-hysteria2-udp-buffer-rollback.json
test ! -e "$sysctl_file"
test ! -e "$ownership_file"
for key in net.core.rmem_default net.core.wmem_default net.core.rmem_max net.core.wmem_max; do
  test "$(sysctl -n "$key")" = "$baseline_bytes"
done
systemctl is-active --quiet vpskit-sing-box.service

/usr/local/bin/vpskit hysteria2 udp-buffer apply --profile conservative-2mib --yes >/root/vpskit-hysteria2-udp-buffer-final.json
for key in net.core.rmem_default net.core.wmem_default net.core.rmem_max net.core.wmem_max; do
  test "$(sysctl -n "$key")" = "$target_bytes"
done
ss -u -a -m -n | awk -v target="$target_bytes" '
  $4 ~ /:443$/ {
    if (getline && $0 ~ ("rb" target) && $0 ~ ("tb" target)) found = 1
  }
  END { exit(found ? 0 : 1) }
'
test "$state_before" = "$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
test "$publication_before" = "$(sha256sum /var/lib/vpskit/subscription-state.json | awk '{print $1}')"
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
printf 'HYSTERIA2_UDP_BUFFER_MANAGED=PASS profile=conservative-2mib\n'
