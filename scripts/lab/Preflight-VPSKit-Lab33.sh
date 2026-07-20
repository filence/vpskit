#!/usr/bin/env bash
set -Eeuo pipefail

for candidate in \
    /root/vpskit-bootstrap-lab33-install.sh \
    /root/v0.1.0-lab.33-linux-amd64.tar.gz \
    /root/vpskit-lab33-menu-smoke; do
    if [ -e "$candidate" ]; then
        printf 'LAB33_REMOTE_PATHS=CLEAR_FALSE\n' >&2
        exit 1
    fi
done

printf 'LAB33_REMOTE_PATHS=CLEAR_TRUE\n'
