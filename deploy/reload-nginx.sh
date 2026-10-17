#!/bin/sh
set -eu
if [ "${RENEWED_LINEAGE:-}" = /etc/letsencrypt/live/artex.noog2infra.site ]; then
    /usr/sbin/nginx -t
    /usr/bin/systemctl reload nginx
fi
