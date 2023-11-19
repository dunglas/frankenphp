#!/bin/sh

set -o errexit
trap 'echo "Aborting due to errexit on line $LINENO. Exit code: $?" >&2' ERR
set -o errtrace
set -o pipefail
set -o xtrace

if ! type "git" > /dev/null; then
    echo "The \"git\" command must be installed."
    exit 1
fi

export CURRENT_REF="$(git rev-parse --abbrev-ref HEAD)"

if [ -n "$1" ]; then
    export TAG="1"
    export FRANKENPHP_VERSION="$1"
    git checkout $FRANKENPHP_VERSION
else
    export TAG="0"
    export FRANKENPHP_VERSION="$(git rev-parse --verify HEAD)"
fi

export PHP_EXTENSIONS="calendar,ctype,curl,dba,dom,exif,filter,fileinfo,gd,iconv,intl,mbstring,mbregex,mysqli,mysqlnd,opcache,openssl,pcntl,pdo,pdo_mysql,pdo_pgsql,pdo_sqlite,pgsql,phar,posix,readline,redis,session,simplexml,sockets,sqlite3,tokenizer,xml,xmlreader,xmlwriter,zip,zlib,apcu"

brew install --formula --quiet  automake cmake composer go gh

mkdir -p dist/
cd dist/

if [ -d "static-php-cli/" ]; then
    cd static-php-cli/
    git pull
else
    git clone --depth 1 https://github.com/crazywhalecc/static-php-cli
    cd static-php-cli/
fi

composer install --no-dev -a
./bin/spc fetch --for-extensions="$PHP_EXTENSIONS"
./bin/spc build --enable-zts --build-embed "$PHP_EXTENSIONS"
export CGO_CFLAGS="-DFRANKENPHP_VERSION=$FRANKENPHP_VERSION $(./buildroot/bin/php-config --includes | sed s#-I/#-I$PWD/buildroot/#g)"
export CGO_LDFLAGS="-framework CoreFoundation -framework SystemConfiguration $(./buildroot/bin/php-config --ldflags) $(./buildroot/bin/php-config --libs)"
export PHP_VERSION="$(./buildroot/bin/php-config --version)"

cd ../../caddy/frankenphp/
go env
go build -buildmode=pie -tags "cgo netgo osusergo static_build" -ldflags "-linkmode=external -extldflags -static-pie -w -s -X 'github.com/caddyserver/caddy/v2.CustomVersion=FrankenPHP $FRANKENPHP_VERSION PHP $PHP_VERSION Caddy'" -o ../../dist/frankenphp-mac-arm64

cd ../../dist/
./frankenphp-mac-arm64 version

if  [ "$TAG" -eq "1" ]; then
    gh release upload $FRANKENPHP_VERSION frankenphp-mac-arm64 --repo dunglas/frankenphp --clobber
fi

git checkout "$CURRENT_REF"
