#!/usr/bin/env bash
set -euo pipefail

readonly version='v0.2.2-lab.2'
readonly archive="/root/${version}-linux-amd64.tar.gz"
readonly root="/root/vpskit-${version}-upgrade"
readonly bundle="${root}/${version}"
readonly backup="/var/lib/vpskit/backups/${version}-cli-upgrade"
readonly backup_binary="${backup}/vpskit-before-${version}"

test -f "$archive"
test -x /usr/local/bin/vpskit
if [ -e "$root" ] || [ -e "$backup_binary" ]; then
  printf 'UPGRADE_REFUSED existing_lab_target\n' >&2
  exit 1
fi
mkdir -p -m 0700 "$root" "$backup"
tar -xzf "$archive" -C "$root"
"$bundle/vpskit" bundle verify --dir "$bundle"
install -m 0700 /usr/local/bin/vpskit "$backup_binary"
install -m 0755 "$bundle/vpskit" /usr/local/bin/vpskit
/usr/local/bin/vpskit version
/usr/local/bin/vpskit migrate plan
/usr/local/bin/vpskit status
printf 'UPGRADE_V022_LAB2=PASS backup=%s\n' "$backup_binary"
