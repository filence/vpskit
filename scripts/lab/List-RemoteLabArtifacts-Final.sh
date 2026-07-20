#!/usr/bin/env bash
set -Eeuo pipefail

find /root -maxdepth 1 \( -type f -o -type d \) \
    \( -name 'v0.1.0-lab.*-linux-amd64.tar.gz' -o -name 'vpskit-lab*' \) \
    -printf '%y %s %p\n' | sort
