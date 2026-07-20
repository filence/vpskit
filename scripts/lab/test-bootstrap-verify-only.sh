#!/usr/bin/env bash
set -Eeuo pipefail

readonly bootstrap_path='/root/vpskit-bootstrap-lab32-install.sh'
readonly archive_path='/root/v0.1.0-lab.32-linux-amd64.tar.gz'

cleanup() {
    rm -f -- "$bootstrap_path" "$archive_path"
}
trap cleanup EXIT HUP INT TERM

[ -f "$bootstrap_path" ] || {
    printf 'BOOTSTRAP_VERIFY=FAIL reason=missing-bootstrap\n' >&2
    exit 1
}
[ -f "$archive_path" ] || {
    printf 'BOOTSTRAP_VERIFY=FAIL reason=missing-archive\n' >&2
    exit 1
}

bash "$bootstrap_path" --archive "$archive_path" --verify-only
printf 'BOOTSTRAP_REMOTE_VERIFY=PASS\n'
