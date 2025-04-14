#!/bin/bash

set -o errexit
set -x

# Ensure required tools are installed
if ! command -v rpmbuild &>/dev/null; then
	echo "Error: rpm-build is required to create RPM packages."
	echo "Install it with: sudo dnf install rpm-build"
	exit 1
fi

if ! command -v ruby &>/dev/null; then
	echo "Error: Ruby is required by FPM."
	echo "Install it with: sudo dnf install ruby"
	exit 1
fi

if ! command -v fpm &>/dev/null; then
	echo "Error: FPM (rubygem-fpm) is required to create RPM packages."
	echo "Install it with: sudo gem install fpm"
	exit 1
fi

arch="$(uname -m)"
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
bin="frankenphp-${os}-${arch}"

if [ ! -f "dist/$bin" ]; then
	echo "Error: dist/$bin not found. Run './build-static.sh' first"
	exit 1
fi

if [[ ! "${FRANKENPHP_VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	echo "Error: FRANKENPHP_VERSION must be set to X.Y.Z (e.g. 1.5.1), got '${FRANKENPHP_VERSION}'"
	exit 1
fi

iteration="1"
cd dist
glibc_version=$(ldd -v "$bin" | awk '/GLIBC_/ {gsub(/[()]/, "", $2); print $2}' | grep -v GLIBC_PRIVATE | sort -V | tail -n1)
cxxabi_version=$(strings "$bin" | grep -oP 'CXXABI_\d+\.\d+(\.\d+)?' | sort -V | tail -n1)

fpm -s dir -t rpm -n frankenphp -v "${FRANKENPHP_VERSION}" \
	--config-files /etc/frankenphp/Caddyfile \
	--config-files /etc/frankenphp/php.ini \
	--depends "libc.so.6(${glibc_version})(64bit)" \
	--depends "libstdc++.so.6(${cxxabi_version})(64bit)" \
	"$bin=/usr/bin/frankenphp" \
	"../package/frankenphp.service=/usr/lib/systemd/system/frankenphp.service" \
	"../package/Caddyfile=/etc/frankenphp/Caddyfile" \
	"../package/etc/php.ini=/etc/frankenphp/php.ini" \
	"../package/etc/php.d/=/etc/frankenphp/php.d" \
	"../package/content/=/usr/share/frankenphp" \
	"../package/modules/=/usr/lib/frankenphp/modules"

rpm_file="frankenphp-${FRANKENPHP_VERSION}-${iteration}.${arch}.rpm"

fpm -s rpm -t deb "$rpm_file"

cd ..
