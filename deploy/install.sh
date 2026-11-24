#!/bin/bash
set -euo pipefail
release_dir="${1:?Usage: install.sh /opt/brm/releases/RELEASE}"
test -x "$release_dir/brm"
install -m 644 /root/brm-deploy/brm.service /etc/systemd/system/brm.service
# --------- новый код chat: отключение прежней системы локальных копий при установке
for unit in brm-backup.timer brm-backup.service; do
    if systemctl cat "$unit" >/dev/null 2>&1; then
        systemctl disable --now "$unit"
    fi
done
rm -f /etc/systemd/system/brm-backup.timer /etc/systemd/system/brm-backup.service /usr/local/sbin/brm-backup
# --------- конец нового кода chat
if [ ! -e /etc/brm.env ]; then
    umask 077
    printf 'APP_ENV=production\nHTTP_ADDR=127.0.0.1:8080\nMAIL_ENABLED=false\nJWT_SECRET=%s\n' "$(openssl rand -hex 48)" > /etc/brm.env
fi
ln -sfn "$release_dir" /opt/brm/current
systemctl daemon-reload
systemctl enable --now brm
systemctl restart brm
systemctl is-active brm
