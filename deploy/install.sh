#!/bin/bash
set -euo pipefail
release_dir="${1:?Usage: install.sh /opt/brm/releases/RELEASE}"
test -x "$release_dir/brm"
install -m 644 /root/brm-deploy/brm.service /etc/systemd/system/brm.service
install -m 644 /root/brm-deploy/brm-backup.service /etc/systemd/system/brm-backup.service
install -m 644 /root/brm-deploy/brm-backup.timer /etc/systemd/system/brm-backup.timer
install -m 700 /root/brm-deploy/brm-backup /usr/local/sbin/brm-backup
if [ ! -e /etc/brm.env ]; then
    umask 077
    printf 'APP_ENV=production\nHTTP_ADDR=127.0.0.1:8080\nMAIL_ENABLED=false\nJWT_SECRET=%s\n' "$(openssl rand -hex 48)" > /etc/brm.env
fi
ln -sfn "$release_dir" /opt/brm/current
systemctl daemon-reload
systemctl enable --now brm
systemctl restart brm
systemctl enable --now brm-backup.timer
systemctl start brm-backup.service
systemctl is-active brm
