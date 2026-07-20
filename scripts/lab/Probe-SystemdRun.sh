#!/usr/bin/env bash
set -eu

printf 'timeout_path='; command -v timeout
printf 'systemd_run_path='; command -v systemd-run
systemd-run --quiet --wait --pipe --collect \
    --unit="vpskit-systemd-probe-$$" \
    -p User=nobody \
    -p Group=nogroup \
    /usr/bin/id
echo 'SYSTEMD_RUN_PROBE=PASS'
