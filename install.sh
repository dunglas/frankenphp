#!/bin/sh

set -e

if [ -z "$BIN_DIR" ]; then
  BIN_DIR=$(pwd)
fi

THE_ARCH_BIN=""
THIS_PROJECT_OWNER="dunglas"
DEST=$BIN_DIR/frankenphp

THISOS=$(uname -s)
ARCH=$(uname -m)

case $THISOS in
   Linux*)
      case $ARCH in
        arm64)
          THE_ARCH_BIN=""
          ;;
        aarch64)
          THE_ARCH_BIN="$THIS_PROJECT_NAME-linux-aarch64"
          ;;
        armv6l)
          THE_ARCH_BIN=""
          ;;
        armv7l)
          THE_ARCH_BIN=""
          ;;
        *)
          THE_ARCH_BIN="$THIS_PROJECT_NAME-linux-x86_64"
          ;;
      esac
      ;;
   Darwin*)
      case $ARCH in
        arm64)
          THE_ARCH_BIN="$THIS_PROJECT_NAME-mac-arm64"
          ;;
        *)
          THE_ARCH_BIN="$THIS_PROJECT_NAME-mac-x86_64"
          ;;
      esac
      ;;
   Windows|MINGW64_NT*)
        THE_ARCH_BIN=""
      ;;
esac

if [ -z "$THE_ARCH_BIN" ]; then
   echo "This script is not supported on $THISOS and $ARCH"
   exit 1
fi

curl -L --progress-bar "https://github.com/$THIS_PROJECT_OWNER/$THIS_PROJECT_NAME/releases/latest/download/$THE_ARCH_BIN" -o "$DEST"

chmod +x "$DEST"

echo "Installed successfully to: $DEST"

