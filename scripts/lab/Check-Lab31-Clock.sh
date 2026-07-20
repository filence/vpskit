#!/usr/bin/env bash
set -euo pipefail
date -u '+REMOTE_UTC=%Y-%m-%dT%H:%M:%SZ'
timedatectl show -p NTPSynchronized --value | sed 's/^/NTP_SYNCHRONIZED=/'
