#!/usr/bin/env bash
set -Eeuo pipefail

for candidate in \
    /root/vpskit-bootstrap-lab33-install.sh \
    /root/v0.1.0-lab.33-linux-amd64.tar.gz \
    /root/vpskit-lab33-menu-smoke; do
    if [ -e "$candidate" ]; then
        printf '%s=present\n' "$(basename "$candidate")"
    else
        printf '%s=absent\n' "$(basename "$candidate")"
    fi
done
