#!/usr/bin/env bash
set -euo pipefail

printf 'SERVER_EPOCH=%s\n' "$(date -u +%s)"
if timedatectl show -p NTPSynchronized --value >/dev/null 2>&1; then
  printf 'SERVER_NTP_SYNCHRONIZED=%s\n' "$(timedatectl show -p NTPSynchronized --value)"
else
  printf 'SERVER_NTP_SYNCHRONIZED=unknown\n'
fi
