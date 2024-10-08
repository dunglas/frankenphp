#!/bin/sh

set -e

if [ -z "${BIN_DIR}" ]; then
	BIN_DIR=$(pwd)
fi

THE_ARCH_BIN=""
DEST=${BIN_DIR}/frankenphp

OS=$(uname -s)
ARCH=$(uname -m)

case ${OS} in
Linux*)
	case ${ARCH} in
	aarch64)
		THE_ARCH_BIN="frankenphp-linux-aarch64"
		;;
	x86_64)
		THE_ARCH_BIN="frankenphp-linux-x86_64"
		;;
	*)
		THE_ARCH_BIN=""
		;;
	esac
	;;
Darwin*)
	case ${ARCH} in
	arm64)
		THE_ARCH_BIN="frankenphp-mac-arm64"
		;;
	*)
		THE_ARCH_BIN="frankenphp-mac-x86_64"
		;;
	esac
	;;
Windows | MINGW64_NT*)
	echo "Install and use WSL to use FrankenPHP on Windows: https://learn.microsoft.com/windows/wsl/"
	exit 1
	;;
*)
	THE_ARCH_BIN=""
	;;
esac

if [ -z "${THE_ARCH_BIN}" ]; then
	echo "FrankenPHP is not supported on ${OS} and ${ARCH}"
	exit 1
fi

SUDO=""

# check if $DEST is writable and suppress an error message
touch "${DEST}" 2>/dev/null

# we need sudo powers to write to DEST
if [ $? -eq 1 ]; then
	echo "You do not have permission to write to ${DEST}, enter your password to grant sudo powers"
	SUDO="sudo"
fi

${SUDO} curl -L --progress-bar "https://github.com/dunglas/frankenphp/releases/latest/download/${THE_ARCH_BIN}" -o "${DEST}"

${SUDO} chmod +x "${DEST}"

echo "FrankenPHP downloaded successfully to ${DEST}"
echo "Move the binary to /usr/local/bin/ or another directory in your PATH to use it globally: sudo mv ${DEST} /usr/local/bin/"
