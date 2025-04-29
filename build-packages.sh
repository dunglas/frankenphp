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

if [ -z "${FRANKENPHP_VERSION}" ]; then
	FRANKENPHP_VERSION="$(git rev-parse --verify HEAD)"
	export FRANKENPHP_VERSION
elif [ -d ".git/" ]; then
	CURRENT_REF="$(git rev-parse --abbrev-ref HEAD)"
	export CURRENT_REF

	if echo "${FRANKENPHP_VERSION}" | grep -F -q "."; then
		# Tag

		# Trim "v" prefix if any
		FRANKENPHP_VERSION=${FRANKENPHP_VERSION#v}
		export FRANKENPHP_VERSION

		git checkout "v${FRANKENPHP_VERSION}"
	else
		git checkout "${FRANKENPHP_VERSION}"
	fi
fi

if [[ ! "${FRANKENPHP_VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	echo "Warning: FRANKENPHP_VERSION must be set to X.Y.Z (e.g. 1.5.1), got '${FRANKENPHP_VERSION}'"
	echo "Falling back to non-release version 0.0.0"
	FRANKENPHP_VERSION=0.0.0
fi

group_preexists=0
user_preexists=0

if getent group frankenphp >/dev/null; then
	group_preexists=1
else
	groupadd --system frankenphp
fi

if getent passwd frankenphp >/dev/null; then
	user_preexists=1
else
	useradd --system \
		--gid frankenphp \
		--create-home \
		--home-dir /var/lib/frankenphp \
		--shell /usr/sbin/nologin \
		--comment "FrankenPHP web server" \
		frankenphp
fi

mkdir -p package/empty
mkdir -p package/etc
[ -f ./dist/static-php-cli/source/php-src/php.ini-production ] && cp -f ./dist/static-php-cli/source/php-src/php.ini-production ./package/etc/php.ini

cd dist
iteration=1
glibc_version=$(ldd -v "$bin" | awk '/GLIBC_/ {gsub(/[()]/, "", $2); print $2}' | grep -v GLIBC_PRIVATE | sort -V | tail -n1)
cxxabi_version=$(strings "$bin" | grep -oP 'CXXABI_\d+\.\d+(\.\d+)?' | sort -V | tail -n1)

fpm -s dir -t rpm -n frankenphp -v "${FRANKENPHP_VERSION}" \
	--config-files /etc/frankenphp/Caddyfile \
	--config-files /etc/frankenphp/php.ini \
	--depends "libc.so.6(${glibc_version})(64bit)" \
	--depends "libstdc++.so.6(${cxxabi_version})(64bit)" \
	--before-install ../package/rhel/preinstall.sh \
	--after-install ../package/rhel/postinstall.sh \
	--before-remove ../package/rhel/preuninstall.sh \
	--after-remove ../package/rhel/postuninstall.sh \
	--iteration "${iteration}" \
	--rpm-user frankenphp --rpm-group frankenphp \
	"${bin}=/usr/bin/frankenphp" \
	"../package/rhel/frankenphp.service=/usr/lib/systemd/system/frankenphp.service" \
	"../package/Caddyfile=/etc/frankenphp/Caddyfile" \
	"../package/content/=/usr/share/frankenphp" \
	"../package/etc/php.ini=/etc/frankenphp/php.ini" \
	"../package/empty/=/etc/frankenphp/php.d" \
	"../package/empty/=/usr/lib/frankenphp/modules" \
	"../package/empty/=/var/lib/frankenphp"

glibc_version=$(ldd -v "$bin" | awk '/GLIBC_/ {gsub(/[()]/, "", $2); print $2}' | grep -v GLIBC_PRIVATE | sed 's/GLIBC_//' | sort -V | tail -n1)
cxxabi_version=$(strings "$bin" | grep -oP 'CXXABI_\d+\.\d+(\.\d+)?' | sed 's/CXXABI_//' | sort -V | tail -n1)

fpm -s dir -t deb -n frankenphp -v "${FRANKENPHP_VERSION}" \
	--config-files /etc/frankenphp/Caddyfile \
	--config-files /etc/frankenphp/php.ini \
	--depends "libc6 (>= ${glibc_version})" \
	--depends "libstdc++6 (>= ${cxxabi_version})" \
	--after-install ../package/debian/postinst.sh \
	--before-remove ../package/debian/prerm.sh \
	--after-remove ../package/debian/postrm.sh \
	--iteration "${iteration}" \
	--deb-user frankenphp --deb-group frankenphp \
	"${bin}=/usr/bin/frankenphp" \
	"../package/debian/frankenphp.service=/usr/lib/systemd/system/frankenphp.service" \
	"../package/Caddyfile=/etc/frankenphp/Caddyfile" \
	"../package/content/=/usr/share/frankenphp" \
	"../package/etc/php.ini=/etc/frankenphp/php.ini" \
	"../package/empty/=/etc/frankenphp/php.d" \
	"../package/empty/=/usr/lib/frankenphp/modules" \
	"../package/empty/=/var/lib/frankenphp"

[ "$user_preexists" -eq 0 ] && userdel frankenphp
[ "$group_preexists" -eq 0 ] && groupdel frankenphp

cd ..
