#!/usr/bin/env bash
set +e
echo "VPSKIT_BIN=$(if [ -x /usr/local/bin/vpskit ]; then /usr/local/bin/vpskit version; else echo absent; fi)"
echo "SERVICE=$(systemctl is-active vpskit-sing-box.service 2>/dev/null || true)"
echo "TIMER=$(systemctl is-active vpskit-certificate-renew.timer 2>/dev/null || true)"
echo "SERVICE_UNIT=$(test -e /etc/systemd/system/vpskit-sing-box.service && echo present || echo absent)"
echo "CONFIG=$(test -e /etc/vpskit/generated/sing-box.json && echo present || echo absent)"
echo "CERT=$(test -e /var/lib/vpskit/certificates/hysteria2.crt && echo present || echo absent)"
echo "TCP443=$(ss -ltnH | grep -c ':443 ' || true)"
echo "UDP443=$(ss -lunH | grep -c ':443 ' || true)"
echo 'JOURNAL_BEGIN'
journalctl -u vpskit-sing-box.service -n 20 --no-pager 2>/dev/null || true
echo 'JOURNAL_END'
