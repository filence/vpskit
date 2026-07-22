#!/usr/bin/env bash
set -euo pipefail

/usr/local/bin/vpskit node modify --tags 'personal,stable,rules-r0006'
/usr/local/bin/vpskit rules show
grep -q 'anti-AD' /etc/vpskit/exports/mihomo.yaml
grep -q 'enhanced-mode: fake-ip' /etc/vpskit/exports/mihomo.yaml
