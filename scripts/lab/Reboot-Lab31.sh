#!/usr/bin/env bash
set -euo pipefail

install -d -m 0700 /root/vpskit-lab31-recovery
cat /proc/sys/kernel/random/boot_id > /root/vpskit-lab31-recovery/pre-reboot-boot-id
chmod 0600 /root/vpskit-lab31-recovery/pre-reboot-boot-id
sync
systemctl reboot
