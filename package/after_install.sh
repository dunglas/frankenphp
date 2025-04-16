#!/bin/bash

if command -v setcap >/dev/null 2>&1; then
	setcap 'cap_net_bind_service=+ep' /usr/bin/frankenphp || echo "Warning: failed to set capabilities on frankenphp"
else
	if [ -f /etc/debian_version ]; then
		echo "Warning: setcap not found. Install it with: sudo apt install libcap2-bin"
	elif [ -f /etc/redhat-release ]; then
		echo "Warning: setcap not found. Install it with: sudo dnf install libcap"
	else
		echo "Warning: setcap not found. Install the appropriate libcap package for your system."
	fi
fi