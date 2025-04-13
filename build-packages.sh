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
	echo "Error: $bin not found. Run './build-static.sh' first"
	exit 1
fi

if [[ ! "${FRANKENPHP_VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	echo "Error: FRANKENPHP_VERSION must be set to X.Y.Z (e.g. 1.5.1), got '${FRANKENPHP_VERSION}'"
	exit 1
fi

cat <<EOF >dist/frankenphp.service
	[Unit]
	Description=FrankenPHP server
	After=network.target

	[Service]
	Type=notify
	User=caddy
	Group=caddy
	ExecStartPre=/usr/bin/frankenphp validate --config /etc/frankenphp/Caddyfile
	ExecStart=/usr/bin/frankenphp run --environ --config /etc/frankenphp/Caddyfile
	ExecReload=/usr/bin/frankenphp reload --config /etc/frankenphp/Caddyfile
	TimeoutStopSec=5s
	LimitNOFILE=1048576
	LimitNPROC=512
	PrivateTmp=true
	ProtectHome=true
	ProtectSystem=full
	AmbientCapabilities=CAP_NET_BIND_SERVICE

	[Install]
	WantedBy=multi-user.target
EOF

cat <<EOF >dist/Caddyfile
	# The Caddyfile is an easy way to configure your Caddy web server.
	#
	# https://caddyserver.com/docs/caddyfile
	# https://frankenphp.dev/docs/config
	{
			# enable the frankenphp module, otherwise "php_server" and "php" directives do not work
			frankenphp {
					# optionally set max_threads, num_threads and create workers here
			}
	}

	http:// {
			root * /usr/share/caddy
			php_server
			file_server
	}

	# As an alternative to editing the above site block, you can add your own site
	# block files in the Caddyfile.d directory, and they will be included as long
	# as they use the .caddyfile extension.

	import Caddyfile.d/*.caddyfile
EOF

iteration="1"
cd dist
glibc_version=$(ldd -v "$bin" | awk '/GLIBC_/ {gsub(/[()]/, "", $2); print $2}' | grep -v GLIBC_PRIVATE | sort -V | tail -n1)
cxxabi_version=$(strings "$bin" | grep -oP 'CXXABI_\d+\.\d+(\.\d+)?' | sort -V | tail -n1)

fpm -s dir -t rpm -n frankenphp -v "${FRANKENPHP_VERSION}" \
	--config-files /etc/frankenphp/Caddyfile \
	--depends "libc.so.6(${glibc_version})(64bit)" \
	--depends "libstdc++.so.6(${cxxabi_version})(64bit)" \
	"$bin=/usr/bin/frankenphp" \
	"./frankenphp.service=/usr/lib/systemd/system/frankenphp.service" \
	"./Caddyfile=/etc/frankenphp/Caddyfile"

rpm_file="frankenphp-${FRANKENPHP_VERSION}-${iteration}.${arch}.rpm"

fpm -s rpm -t deb "$rpm_file"

cd ..
