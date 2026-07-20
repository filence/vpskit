#!/usr/bin/env bash
set +e
for path in /root/v0.1.0-lab.16-linux-amd64.tar.gz /root/.vpskit-cf-token /root/vpskit-lab-v0.1.0-lab.16-clean /root/vpskit-lab-v0.1.0-lab.16-clean/v0.1.0-lab.16; do
    if [ -e "$path" ]; then
        stat -c '%n %F %a %s' "$path"
    else
        echo "$path absent"
    fi
done
