#!/usr/bin/env bash
set -euo pipefail

for script in \
  /root/Deploy-VPSKit-v0.2.3-UserRules-Lab.sh \
  /root/Test-VPSKit-v0.2.3-UserRules.sh \
  /root/Inspect-VPSKit-v0.2.3-UserRules.sh \
  /root/Publish-VPSKit-v0.2.3-UserRules.sh; do
  bash -n "$script"
  printf 'SHELL_PARSE=PASS %s\n' "$script"
done
