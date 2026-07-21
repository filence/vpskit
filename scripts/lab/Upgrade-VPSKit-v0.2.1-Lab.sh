#!/usr/bin/env bash
set -euo pipefail

readonly version='v0.2.1-lab.7'
readonly archive="/root/${version}-linux-amd64.tar.gz"
readonly root="/root/vpskit-${version}-upgrade"
readonly bundle="${root}/${version}"
readonly backup="/var/lib/vpskit/backups/${version}-cli-upgrade"

rm -rf -- "$root"
mkdir -p -- "$root" "$backup"
tar -xzf "$archive" -C "$root"
"$bundle/vpskit" bundle verify --dir "$bundle"
install -m 0700 /usr/local/bin/vpskit "${backup}/vpskit-before-${version}"
install -m 0755 "$bundle/vpskit" /usr/local/bin/vpskit
/usr/local/bin/vpskit version
/usr/local/bin/vpskit status
