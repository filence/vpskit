#!/usr/bin/env bash
set -Eeuo pipefail

targets=(
    '/root/vpskit-lab20-core-update'
    '/root/vpskit-lab21-supply-chain'
    '/root/vpskit-lab22-crash-recovery'
    '/root/vpskit-lab23-crash-recovery'
    '/root/v0.1.0-lab.22-core-crash-linux-amd64.tar.gz'
    '/root/v0.1.0-lab.22-linux-amd64.tar.gz'
)

for target in "${targets[@]}"; do
    case "$target" in
        /root/vpskit-lab20-core-update|\
        /root/vpskit-lab21-supply-chain|\
        /root/vpskit-lab22-crash-recovery|\
        /root/vpskit-lab23-crash-recovery)
            if [ -d "$target" ]; then
                rm -rf -- "$target"
            fi
            ;;
        /root/v0.1.0-lab.22-core-crash-linux-amd64.tar.gz|\
        /root/v0.1.0-lab.22-linux-amd64.tar.gz)
            rm -f -- "$target"
            ;;
        *)
            printf 'REFUSED unexpected target: %s\n' "$target" >&2
            exit 1
            ;;
    esac
done

for target in "${targets[@]}"; do
    test ! -e "$target"
done
/usr/local/bin/vpskit doctor >/dev/null
printf 'REMOTE_LAB_ARTIFACT_CLEANUP=PASS removed=%s\n' "${#targets[@]}"
