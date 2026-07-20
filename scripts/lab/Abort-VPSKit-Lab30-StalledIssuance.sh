#!/usr/bin/env bash
set -Eeuo pipefail

mapfile -t lego_pids < <(pgrep -f '^/root/vpskit-lab30-run/lab30/v0\.1\.0-lab\.30/lego run ' || true)
test "${#lego_pids[@]}" -eq 1
kill -TERM "${lego_pids[0]}"

for _ in $(seq 1 60); do
    if ! pgrep -f '^/root/vpskit-lab30-run/lab30/v0\.1\.0-lab\.30/vpskit install balanced ' >/dev/null; then
        break
    fi
    sleep 1
done

if pgrep -f '^/root/vpskit-lab30-run/lab30/v0\.1\.0-lab\.30/vpskit install balanced ' >/dev/null; then
    printf 'LAB30_STALLED_PROCESS=STILL_RUNNING\n' >&2
    exit 1
fi
test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.29'
systemctl is-active --quiet vpskit-sing-box.service
/usr/local/bin/vpskit doctor >/dev/null
printf 'LAB30_STALLED_ISSUANCE_ABORT_AND_RECOVERY=PASS\n'
