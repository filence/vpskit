#!/usr/bin/env bash
set -Eeuo pipefail

runtime='/root/vpskit-xray-compat-runtime'
upload='/root/vpskit-xray-compat-xray'
unit='vpskit-xray-compat.service'
legacy_runtime='/run/vpskit-xray-compat'

systemctl stop "$unit" >/dev/null 2>&1 || true
systemctl reset-failed "$unit" >/dev/null 2>&1 || true
case "$legacy_runtime" in
  /run/vpskit-xray-compat) rm -rf -- "$legacy_runtime" ;;
  *) printf 'refusing unexpected legacy compatibility runtime path\n' >&2; exit 1 ;;
esac
case "$runtime" in
  /root/vpskit-xray-compat-runtime) rm -rf -- "$runtime" ;;
  *) printf 'refusing unexpected compatibility runtime path\n' >&2; exit 1 ;;
esac
case "$upload" in
  /root/vpskit-xray-compat-xray) rm -f -- "$upload" ;;
  *) printf 'refusing unexpected compatibility upload path\n' >&2; exit 1 ;;
esac
printf 'XRAY_COMPAT_CLEANUP=PASS\n'
