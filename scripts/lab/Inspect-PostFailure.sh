#!/usr/bin/env bash
set -eu

latest="$(find /var/lib/vpskit/transactions -mindepth 2 -maxdepth 2 -name transaction.json -printf '%T@ %p\n' 2>/dev/null | sort -n | tail -n 1 | cut -d' ' -f2-)"
printf 'latest_transaction_status='
if [ -n "$latest" ]; then
    sed -n 's/^[[:space:]]*"status":[[:space:]]*"\([^"]*\)".*/\1/p' "$latest"
else
    echo absent
fi
printf 'service_user='; if id vpskit >/dev/null 2>&1; then echo present; else echo absent; fi
printf 'service_unit='; if [ -e /etc/systemd/system/vpskit-sing-box.service ]; then echo present; else echo absent; fi
printf 'temporary_token='; if [ -e /root/.vpskit-cf-token ]; then echo present; else echo absent; fi
printf 'managed_config='; if [ -e /etc/vpskit/generated/sing-box.json ]; then echo present; else echo absent; fi
printf 'tcp_443_listeners='; ss -ltnH | grep -c ':443 ' || true
printf 'udp_443_listeners='; ss -lunH | grep -c ':443 ' || true
