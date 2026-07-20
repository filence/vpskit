#!/usr/bin/env bash
set -Eeuo pipefail

for candidate in \
    /root/vpskit-bootstrap-lab32-install.sh \
    /root/v0.1.0-lab.32-linux-amd64.tar.gz; do
    if [ -e "$candidate" ]; then
        printf 'BOOTSTRAP_REMOTE_PATHS=CLEAR_FALSE\n' >&2
        exit 1
    fi
done

printf 'BOOTSTRAP_REMOTE_PATHS=CLEAR_TRUE\n'
