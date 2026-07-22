#!/usr/bin/env bash
set -euo pipefail

systemd-run --unit=vpskit-port-hop-subscription-readback /bin/sh -c '/usr/local/bin/vpskit subscription status >/root/vpskit-port-hop-subscription-lifecycle.json 2>&1'
printf '%s\n' 'VPSKIT_PORTHOP_SUBSCRIPTION_READBACK_STARTED=PASS'
