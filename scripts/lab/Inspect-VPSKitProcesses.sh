#!/usr/bin/env bash
set +e
echo PROCESSES_BEGIN
ps -eo pid,ppid,stat,etime,args | grep -E 'vpskit|lego|sing-box|Debug-Clean|Deploy-VPSKit' | grep -v grep || true
echo PROCESSES_END
echo LOCK_BEGIN
find /var/lib/vpskit -maxdepth 3 -type f -name '*.lock' -o -type s -name '*.lock' 2>/dev/null | while read -r path; do stat -c '%n %F %a %s' "$path"; done
echo LOCK_END
