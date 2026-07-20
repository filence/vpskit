#!/usr/bin/env bash
set -euo pipefail
journalctl -u vpskit-xray.service --since '20 minutes ago' --no-pager -n 100 \
  | sed -E 's/[0-9a-fA-F]{8}-[0-9a-fA-F-]{27,}/<uuid>/g; s/[A-Za-z0-9_-]{40,}/<redacted>/g'
