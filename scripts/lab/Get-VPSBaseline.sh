#!/usr/bin/env bash
set -eu

lab_domain="${VPSKIT_LAB_DOMAIN:?VPSKIT_LAB_DOMAIN is required}"

printf 'BASELINE_BEGIN\n'
printf 'hostname=%s\n' "$(hostname)"
printf 'kernel=%s\n' "$(uname -srmo)"
printf 'arch=%s\n' "$(uname -m)"
printf 'pid1=%s\n' "$(ps -p 1 -o comm= | tr -d ' ')"

printf '%s\n' 'OS_RELEASE_BEGIN'
cat /etc/os-release
printf '%s\n' 'OS_RELEASE_END'

printf '%s\n' 'UPTIME_BEGIN'
uptime
printf '%s\n' 'UPTIME_END'

printf '%s\n' 'MEMORY_BEGIN'
free -m
printf '%s\n' 'MEMORY_END'

printf '%s\n' 'DISK_BEGIN'
df -hT /
printf '%s\n' 'DISK_END'

printf '%s\n' 'LISTENERS_BEGIN'
ss -ltnup
printf '%s\n' 'LISTENERS_END'

printf '%s\n' 'UNITS_BEGIN'
systemctl list-unit-files --type=service --no-legend \
    | grep -E '^(sing-box|xray|vpskit|nginx|caddy|docker|ufw|firewalld)' \
    || true
printf '%s\n' 'UNITS_END'

printf '%s\n' 'FIREWALL_BEGIN'
if command -v ufw >/dev/null 2>&1; then ufw status verbose; else echo 'ufw=absent'; fi
if command -v firewall-cmd >/dev/null 2>&1; then firewall-cmd --state; else echo 'firewalld=absent'; fi
if command -v nft >/dev/null 2>&1; then nft list ruleset; else echo 'nft=absent'; fi
printf '%s\n' 'FIREWALL_END'

printf '%s\n' 'TIME_BEGIN'
timedatectl show -p NTPSynchronized -p TimeUSec -p Timezone 2>/dev/null || true
printf '%s\n' 'TIME_END'

printf '%s\n' 'DNS_BEGIN'
getent ahosts "$lab_domain" || true
printf '%s\n' 'DNS_END'

printf '%s\n' 'TOOLS_BEGIN'
for command_name in curl tar sha256sum openssl systemctl ss; do
    if command -v "$command_name" >/dev/null 2>&1; then
        printf '%s=present\n' "$command_name"
    else
        printf '%s=absent\n' "$command_name"
    fi
done
printf '%s\n' 'TOOLS_END'
printf 'BASELINE_END\n'
