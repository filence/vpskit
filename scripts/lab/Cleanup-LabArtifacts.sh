#!/usr/bin/env bash
set -euo pipefail

targets=(
    '/root/v0.1.0-lab.1-linux-amd64.tar.gz'
    '/root/vpskit-lab-v0.1.0-lab.1'
    '/root/v0.1.0-lab.2-linux-amd64.tar.gz'
    '/root/vpskit-lab-v0.1.0-lab.2'
    '/root/v0.1.0-lab.3-linux-amd64.tar.gz'
    '/root/vpskit-lab-v0.1.0-lab.3'
    '/root/v0.1.0-lab.4-linux-amd64.tar.gz'
    '/root/vpskit-lab-v0.1.0-lab.4'
    '/root/vpskit-v0.1.0-lab.5-doctor'
    '/root/v0.1.0-lab.6-linux-amd64.tar.gz'
    '/root/vpskit-lab-v0.1.0-lab.6-scan'
    '/root/Test-VPSKit-Lab6-RealityScanner.sh'
    '/root/v0.1.0-lab.7-linux-amd64.tar.gz'
    '/root/vpskit-lab-v0.1.0-lab.7-cert'
    '/root/Test-VPSKit-Lab7-CertificateLifecycle.sh'
    '/root/v0.1.0-lab.8-linux-amd64.tar.gz'
    '/root/vpskit-lab-v0.1.0-lab.8-cert'
    '/root/Test-VPSKit-Lab8-CertificateLifecycle.sh'
    '/root/v0.1.0-lab.9-linux-amd64.tar.gz'
    '/root/vpskit-lab-v0.1.0-lab.9-update'
    '/root/Test-VPSKit-Lab9-UpdateMigration.sh'
    '/root/v0.1.0-lab.10-linux-amd64.tar.gz'
    '/root/vpskit-lab-v0.1.0-lab.10-backup'
    '/root/Test-VPSKit-Lab10-BackupRestore.sh'
    '/root/v0.1.0-lab.11-linux-amd64.tar.gz'
    '/root/vpskit-lab-v0.1.0-lab.11-backup'
    '/root/Test-VPSKit-Lab11-BackupRestore.sh'
    '/root/v0.1.0-lab.13-linux-amd64.tar.gz'
    '/root/vpskit-lab-v0.1.0-lab.13-backup'
    '/root/Test-VPSKit-Lab13-BackupRestore.sh'
    '/root/v0.1.0-lab.14-linux-amd64.tar.gz'
    '/root/vpskit-lab-v0.1.0-lab.14-rollback'
    '/root/Test-VPSKit-Lab14-Rollback.sh'
    '/root/v0.1.0-lab.15-linux-amd64.tar.gz'
    '/root/vpskit-lab-v0.1.0-lab.15-rollback'
    '/root/Test-VPSKit-Lab15-Rollback.sh'
    '/root/v0.1.0-lab.16-linux-amd64.tar.gz'
    '/root/vpskit-lab-v0.1.0-lab.16-uninstall-gate'
    '/root/Test-VPSKit-Lab16-UninstallGate.sh'
)

existing=()
for target in "${targets[@]}"; do
    if [ ! -e "$target" ] && [ ! -L "$target" ]; then
        continue
    fi
    resolved="$(readlink -f -- "$target")"
    if [ "$resolved" != "$target" ]; then
        echo "CLEANUP_REFUSED unexpected_resolution=$target" >&2
        exit 1
    fi
    case "$target" in
        /root/v0.1.0-lab.[1-4]-linux-amd64.tar.gz|/root/vpskit-lab-v0.1.0-lab.[1-4]|/root/vpskit-v0.1.0-lab.5-doctor|/root/v0.1.0-lab.6-linux-amd64.tar.gz|/root/vpskit-lab-v0.1.0-lab.6-scan|/root/Test-VPSKit-Lab6-RealityScanner.sh|/root/v0.1.0-lab.7-linux-amd64.tar.gz|/root/vpskit-lab-v0.1.0-lab.7-cert|/root/Test-VPSKit-Lab7-CertificateLifecycle.sh|/root/v0.1.0-lab.8-linux-amd64.tar.gz|/root/vpskit-lab-v0.1.0-lab.8-cert|/root/Test-VPSKit-Lab8-CertificateLifecycle.sh|/root/v0.1.0-lab.9-linux-amd64.tar.gz|/root/vpskit-lab-v0.1.0-lab.9-update|/root/Test-VPSKit-Lab9-UpdateMigration.sh|/root/v0.1.0-lab.10-linux-amd64.tar.gz|/root/vpskit-lab-v0.1.0-lab.10-backup|/root/Test-VPSKit-Lab10-BackupRestore.sh|/root/v0.1.0-lab.11-linux-amd64.tar.gz|/root/vpskit-lab-v0.1.0-lab.11-backup|/root/Test-VPSKit-Lab11-BackupRestore.sh|/root/v0.1.0-lab.13-linux-amd64.tar.gz|/root/vpskit-lab-v0.1.0-lab.13-backup|/root/Test-VPSKit-Lab13-BackupRestore.sh|/root/v0.1.0-lab.14-linux-amd64.tar.gz|/root/vpskit-lab-v0.1.0-lab.14-rollback|/root/Test-VPSKit-Lab14-Rollback.sh|/root/v0.1.0-lab.15-linux-amd64.tar.gz|/root/vpskit-lab-v0.1.0-lab.15-rollback|/root/Test-VPSKit-Lab15-Rollback.sh|/root/v0.1.0-lab.16-linux-amd64.tar.gz|/root/vpskit-lab-v0.1.0-lab.16-uninstall-gate|/root/Test-VPSKit-Lab16-UninstallGate.sh)
            existing+=("$target")
            ;;
        *)
            echo "CLEANUP_REFUSED unexpected_target=$target" >&2
            exit 1
            ;;
    esac
done

for target in "${existing[@]}"; do
    rm -rf -- "$target"
done

remaining=0
for target in "${targets[@]}"; do
    if [ -e "$target" ] || [ -L "$target" ]; then
        remaining=$((remaining + 1))
    fi
done
test "$remaining" -eq 0
test ! -e /root/.vpskit-cf-token
echo "LAB_ARTIFACT_CLEANUP=PASS removed=${#existing[@]} remaining=0 token_staging=absent"
