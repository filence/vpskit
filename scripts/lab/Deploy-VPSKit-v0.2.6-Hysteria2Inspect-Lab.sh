#!/usr/bin/env bash
set -euo pipefail

readonly version='v0.2.6-lab.1'
readonly archive="/root/${version}-linux-amd64.tar.gz"
readonly archive_sha256='73703b007a1378e44451d3cd96f6e6908acb1f09ce0d3b1296abda955cb9b24d'
readonly root="/root/vpskit-${version}-upgrade"
readonly bundle="${root}/${version}"
readonly backup="/var/lib/vpskit/backups/${version}-cli-upgrade"
readonly backup_binary="${backup}/vpskit-before-${version}"

test -f "$archive"
test -x /usr/local/bin/vpskit
test "$(sha256sum "$archive" | awk '{print $1}')" = "$archive_sha256"
if [ -e "$root" ] || [ -e "$backup_binary" ]; then
  printf 'UPGRADE_REFUSED existing_lab_target\n' >&2
  exit 1
fi

state_before="$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
readonly state_before
publication_before="$(sha256sum /var/lib/vpskit/subscription-state.json | awk '{print $1}')"
readonly publication_before
install -d -m 0700 "$root"
install -d -m 0700 "$backup"
tar -xzf "$archive" -C "$root"
"$bundle/vpskit" bundle verify --dir "$bundle"
install -m 0700 /usr/local/bin/vpskit "$backup_binary"
install -m 0755 "$bundle/vpskit" /usr/local/bin/vpskit
test "$(/usr/local/bin/vpskit version)" = "vpskit ${version}"

/usr/local/bin/vpskit hysteria2 inspect
test "$state_before" = "$(sha256sum /var/lib/vpskit/state.json | awk '{print $1}')"
test "$publication_before" = "$(sha256sum /var/lib/vpskit/subscription-state.json | awk '{print $1}')"
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
printf 'HYSTERIA2_INSPECT_READ_ONLY=PASS\n'
