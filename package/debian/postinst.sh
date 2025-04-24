#!/bin/sh
set -e

if [ "$1" = "configure" ]; then
        # Add user and group
        if ! getent group frankenphp >/dev/null; then
                groupadd --system frankenphp
        fi
        if ! getent passwd frankenphp >/dev/null; then
                useradd --system \
                        --gid frankenphp \
                        --create-home \
                        --home-dir /var/lib/frankenphp \
                        --shell /usr/sbin/nologin \
                        --comment "FrankenPHP web server" \
                        frankenphp
        fi
        if getent group www-data >/dev/null; then
                usermod -aG www-data frankenphp
        fi

        # handle cases where package was installed and then purged;
        # user and group will still exist but with no home dir
        if [ ! -d /var/lib/frankenphp ]; then
                mkdir -p /var/lib/frankenphp
                chown frankenphp:frankenphp /var/lib/frankenphp
        fi

        # Add log directory with correct permissions
        if [ ! -d /var/log/frankenphp ]; then
                mkdir -p /var/log/frankenphp
                chown frankenphp:frankenphp /var/log/frankenphp
        fi
fi

if [ "$1" = "configure" ] || [ "$1" = "abort-upgrade" ] || [ "$1" = "abort-deconfigure" ] || [ "$1" = "abort-remove" ] ; then
        # This will only remove masks created by d-s-h on package removal.
        deb-systemd-helper unmask frankenphp.service >/dev/null || true

        # was-enabled defaults to true, so new installations run enable.
        if deb-systemd-helper --quiet was-enabled frankenphp.service; then
                # Enables the unit on first installation, creates new
                # symlinks on upgrades if the unit file has changed.
                deb-systemd-helper enable frankenphp.service >/dev/null || true
                deb-systemd-invoke start frankenphp.service >/dev/null || true
        else
                # Update the statefile to add new symlinks (if any), which need to be
                # cleaned up on purge. Also remove old symlinks.
                deb-systemd-helper update-state frankenphp.service >/dev/null || true
        fi

        # Restart only if it was already started
        if [ -d /run/systemd/system ]; then
                systemctl --system daemon-reload >/dev/null || true
                if [ -n "$2" ]; then
                        deb-systemd-invoke try-restart frankenphp.service >/dev/null || true
                fi
        fi
fi
