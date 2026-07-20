#!/usr/bin/env bash
set -eu

bundle='/root/vpskit-lab-v0.1.0-lab.1/v0.1.0-lab.1'
printf '%s\n' 'LEGO_RUN_HELP_BEGIN'
"$bundle/lego" run --help
printf '%s\n' 'LEGO_RUN_HELP_END'
printf '%s\n' 'ROLLBACK_STATE_BEGIN'
find /var/lib/vpskit -maxdepth 4 -printf '%M %u:%g %p\n' 2>/dev/null | sort || true
for transaction in /var/lib/vpskit/transactions/*/transaction.json; do
    if [ -f "$transaction" ]; then
        cat "$transaction"
    fi
done
printf '%s\n' 'ROLLBACK_STATE_END'
printf 'service_user='; if id vpskit >/dev/null 2>&1; then echo present; else echo absent; fi
printf 'service_unit='; if [ -e /etc/systemd/system/vpskit-sing-box.service ]; then echo present; else echo absent; fi
printf '%s\n' 'LISTENERS_BEGIN'
ss -ltnup | grep ':443' || true
printf '%s\n' 'LISTENERS_END'
