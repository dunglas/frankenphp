#!/bin/sh
set -e

if [ -d /run/systemd/system ] && [ "$1" = remove ]; then
        deb-systemd-invoke stop frankenphp.service >/dev/null || true
        deb-systemd-invoke stop frankenphp-api.service >/dev/null || true
fi
