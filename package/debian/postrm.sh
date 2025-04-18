#!/bin/sh
set -e

if [ -d /run/systemd/system ]; then
	systemctl --system daemon-reload >/dev/null || true
fi

if [ "$1" = "remove" ]; then
	if [ -x "/usr/bin/deb-systemd-helper" ]; then
		deb-systemd-helper mask frankenphp.service >/dev/null || true
	fi
fi

if [ "$1" = "purge" ]; then
	if [ -x "/usr/bin/deb-systemd-helper" ]; then
		deb-systemd-helper purge frankenphp.service >/dev/null || true
		deb-systemd-helper unmask frankenphp.service >/dev/null || true
	fi
	rm -rf /var/lib/frankenphp /var/log/frankenphp /etc/frankenphp
fi
