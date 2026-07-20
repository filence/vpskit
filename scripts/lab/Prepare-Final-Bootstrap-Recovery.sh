#!/usr/bin/env bash
set -Eeuo pipefail

readonly recovery_root='/root/vpskit-final-bootstrap-lab33'
readonly snapshot='/root/vpskit-final-bootstrap-lab33-preinstall.tar.gz'
readonly snapshot_hash='/root/vpskit-final-bootstrap-lab33-preinstall.tar.gz.sha256'
readonly archive='/root/vpskit-final-bootstrap-lab33-release.tar.gz'
readonly installer='/root/vpskit-final-bootstrap-lab33-install.sh'
readonly loopback='/root/vpskit-final-bootstrap-lab33-loopback.sh'

[ "$(id -u)" -eq 0 ]
[ "$(uname -m)" = 'x86_64' ]
grep -q '^ID=debian$' /etc/os-release
grep -q '^VERSION_ID="\?13"\?$' /etc/os-release

for path in "$recovery_root" "$snapshot" "$snapshot_hash" "$archive" "$installer" "$loopback"; do
    if [ -e "$path" ] || [ -L "$path" ]; then
        printf 'FINAL_BOOTSTRAP_PREFLIGHT=FAIL reason=reserved_path_exists\n' >&2
        exit 1
    fi
done

test "$(/usr/local/bin/vpskit version)" = 'vpskit v0.1.0-lab.33'
/usr/local/bin/vpskit doctor >/dev/null
systemctl is-active --quiet vpskit-xray.service
systemctl is-active --quiet vpskit-sing-box.service
systemctl is-active --quiet vpskit-certificate-renew.timer

for path in \
    /var/lib/vpskit/state.json \
    /var/lib/vpskit/ownership.json \
    /var/lib/vpskit/secrets/cloudflare.env \
    /var/lib/vpskit/secrets/acme.env \
    /var/lib/vpskit/certificates/hysteria2.crt \
    /var/lib/vpskit/certificates/hysteria2.key; do
    test -f "$path"
    test ! -L "$path"
done

available_kib="$(df --output=avail -k /root | tail -n 1 | tr -d ' ')"
test "$available_kib" -ge 524288

backup_output="$(/usr/local/bin/vpskit backup)"
backup_id="$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["detail"]["backup_id"])' "$backup_output")"
case "$backup_id" in
    BK-[0-9]*-[0-9]*-[0-9a-f]*) ;;
    *) printf 'FINAL_BOOTSTRAP_PREFLIGHT=FAIL reason=invalid_backup_id\n' >&2; exit 1 ;;
esac
test -d "/var/lib/vpskit/backups/$backup_id"

install -d -o root -g root -m 0700 "$recovery_root"
cp -a -- "/var/lib/vpskit/backups/$backup_id" "$recovery_root/"
install -o root -g root -m 0600 /var/lib/vpskit/secrets/cloudflare.env "$recovery_root/cloudflare.env"
install -o root -g root -m 0600 /var/lib/vpskit/secrets/acme.env "$recovery_root/acme.env"
install -o root -g root -m 0600 /var/lib/vpskit/certificates/hysteria2.crt "$recovery_root/hysteria2.crt"
install -o root -g root -m 0600 /var/lib/vpskit/certificates/hysteria2.key "$recovery_root/hysteria2.key"
printf '%s\n' "$backup_id" >"$recovery_root/backup-id"
getent passwd vpskit >"$recovery_root/passwd-entry"
getent group vpskit >"$recovery_root/group-entry"
chmod 0600 "$recovery_root/backup-id" "$recovery_root/passwd-entry" "$recovery_root/group-entry"

tar -C / -czf "$snapshot" \
    --exclude='var/lib/vpskit/backups' \
    etc/vpskit \
    var/lib/vpskit \
    var/log/vpskit \
    usr/local/bin/vpskit \
    usr/local/lib/vpskit \
    etc/systemd/system/vpskit-sing-box.service \
    etc/systemd/system/vpskit-xray.service \
    etc/systemd/system/vpskit-certificate-renew.service \
    etc/systemd/system/vpskit-certificate-renew.timer \
    root/vpskit-final-bootstrap-lab33
chmod 0600 "$snapshot"
tar -tzf "$snapshot" >/dev/null
sha256sum "$snapshot" | awk '{print $1}' >"$snapshot_hash"
chmod 0600 "$snapshot_hash"

printf 'FINAL_BOOTSTRAP_PREFLIGHT=PASS\n'
printf 'FINAL_BOOTSTRAP_MANAGED_BACKUP=PASS\n'
printf 'FINAL_BOOTSTRAP_RECOVERY_SNAPSHOT=PASS\n'
