#!/bin/bash
set -euo pipefail

# Set paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DIST_DIR="${SCRIPT_DIR}/dist/dependencies"
BUILDROOT_LIB="$DIST_DIR/lib"
BUILDROOT_INCLUDE="$DIST_DIR/include"

# Ensure folders exist
mkdir -p "$BUILDROOT_LIB" "$BUILDROOT_INCLUDE"

# Check for libwatcher-c.a and header
if [ ! -f "$BUILDROOT_LIB/libwatcher-c.a" ] || [ ! -f "$BUILDROOT_INCLUDE/wtr/watcher-c.h" ]; then
	echo "Building libwatcher-c..."
	mkdir -p watcher
	cd watcher
	curl -sL https://api.github.com/repos/e-dant/watcher/releases/latest |
		grep tarball_url |
		awk -F '"' '{print $4}' |
		xargs curl -sL | tar xz --strip-components=1
	cd watcher-c
	${CC:-cc} -c -o libwatcher-c.o ./src/watcher-c.cpp -I ./include -I ../include -std=c++17 -Wall -Wextra -fPIC
	ar rcs libwatcher-c.a libwatcher-c.o
	cp libwatcher-c.a "$BUILDROOT_LIB/"
	mkdir -p "$BUILDROOT_INCLUDE/wtr"
	cp -R include/wtr/watcher-c.h "$BUILDROOT_INCLUDE/wtr/"
	cd ../../
	rm -rf watcher
fi

# Check for Brotli static libs and headers
if [ ! -f "$BUILDROOT_LIB/libbrotlienc.a" ] ||
	[ ! -f "$BUILDROOT_LIB/libbrotlidec.a" ] ||
	[ ! -f "$BUILDROOT_LIB/libbrotlicommon.a" ] ||
	[ ! -d "$BUILDROOT_INCLUDE/brotli" ]; then
	echo "Building Brotli..."
	if ! command -v cmake &>/dev/null; then
		echo "cmake is not installed. Please install cmake to build Brotli."
		exit 1
	fi
	git clone --depth 1 https://github.com/google/brotli.git brotli-source
	cd brotli-source
	mkdir out && cd out
	cmake -DCMAKE_BUILD_TYPE=Release -DCMAKE_POSITION_INDEPENDENT_CODE=ON -DBUILD_SHARED_LIBS=OFF ..
	make -j"$(nproc)"
	cp libbrotlienc.a libbrotlidec.a libbrotlicommon.a "$BUILDROOT_LIB/"
	cp -R ../c/include/brotli "$BUILDROOT_INCLUDE/"
	cd ../../
	rm -rf brotli-source
fi

if ! command -v xcaddy &>/dev/null; then
	echo "Installing xcaddy..."

	go install github.com/caddyserver/xcaddy/cmd/xcaddy@latest

	# Determine install path
	if [ -n "${GOBIN:-}" ] && [ -x "$GOBIN/xcaddy" ]; then
		XCADDY="$GOBIN/xcaddy"
	elif [ -n "${GOPATH:-}" ] && [ -x "$GOPATH/bin/xcaddy" ]; then
		XCADDY="$GOPATH/bin/xcaddy"
	elif [ -x "$HOME/go/bin/xcaddy" ]; then
		XCADDY="$HOME/go/bin/xcaddy"
	else
		echo "Error: xcaddy installed but not found in expected paths." >&2
		echo "Ensure \$GOBIN, \$GOPATH/bin, or \$HOME/go/bin exists and contains xcaddy." >&2
		exit 1
	fi

	echo "xcaddy installed at: $XCADDY"
	EXPORT_CMD="export PATH=\"$(dirname "$XCADDY"):\$PATH\""

	echo "To make xcaddy available, run the following:"
	echo "	echo '$EXPORT_CMD' >> ~/.bashrc && source ~/.bashrc"
else
	XCADDY="$(command -v xcaddy)"
fi
