#!/bin/sh

set -o errexit
trap 'echo "Aborting due to errexit on line $LINENO. Exit code: $?" >&2' ERR
set -o xtrace

if ! type "git" > /dev/null; then
    echo "The \"git\" command must be installed."
    exit 1
fi

if [ -z "$PHP_EXTENSIONS" ]; then
    export PHP_EXTENSIONS="apcu,bcmath,bz2,calendar,ctype,curl,dba,dom,exif,fileinfo,filter,gd,iconv,intl,ldap,mbregex,mbstring,mysqli,mysqlnd,opcache,openssl,pcntl,pdo,pdo_mysql,pdo_pgsql,pdo_sqlite,pgsql,phar,posix,readline,redis,session,simplexml,sockets,sqlite3,sysvsem,tokenizer,xml,xmlreader,xmlwriter,zip,zlib"
fi

if [ -z "$PHP_EXTENSIONS_LIB" ]; then
    export PHP_EXTENSION_LIBS="freetype,libjpeg,libavif,libwebp,libzip,bzip2"
fi

if [ -z "$PHP_VERSION" ]; then
    export PHP_VERSION="8.3"
fi

if [ -z "$FRANKENPHP_VERSION" ]; then
    FRANKENPHP_VERSION="$(git rev-parse --verify HEAD)"
    export FRANKENPHP_VERSION
elif [ -d ".git/" ]; then
    CURRENT_REF="$(git rev-parse --abbrev-ref HEAD)"
    export CURRENT_REF
    git checkout "$FRANKENPHP_VERSION"
fi

arch="$(uname -m)"
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
if [ "$os" = "darwin" ]; then
    os="mac"
fi
bin="frankenphp-$os-$arch"

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


if type "brew" > /dev/null; then
    packages="composer"
    if [ "$RELEASE" ]; then
        packages="$packages gh"
    fi

    brew install --formula --quiet "$packages"
fi

if [ "$os" = "linux" ]; then
    extraOpts="--disable-opcache-jit"
fi

./bin/spc doctor
./bin/spc fetch --with-php="$PHP_VERSION" --for-extensions="$PHP_EXTENSIONS"
./bin/spc build --enable-zts --build-embed --debug $extraOpts "$PHP_EXTENSIONS" --with-libs="$PHP_EXTENSION_LIBS"
CGO_CFLAGS="-DFRANKENPHP_VERSION=$FRANKENPHP_VERSION $(./buildroot/bin/php-config --includes | sed s#-I/#-I"$PWD"/buildroot/#g)"
export CGO_CFLAGS

if [ "$os" = "mac" ]; then
    export CGO_LDFLAGS="-framework CoreFoundation -framework SystemConfiguration"
fi

CGO_LDFLAGS="$CGO_LDFLAGS $(./buildroot/bin/php-config --ldflags) $(./buildroot/bin/php-config --libs)"
export CGO_LDFLAGS

LIBPHP_VERSION="$(./buildroot/bin/php-config --version)"
export LIBPHP_VERSION

cd ../../caddy/frankenphp/
go env
go build -buildmode=pie -tags "cgo netgo osusergo static_build" -ldflags "-linkmode=external -extldflags -static-pie -w -s -X 'github.com/caddyserver/caddy/v2.CustomVersion=FrankenPHP $FRANKENPHP_VERSION PHP $LIBPHP_VERSION Caddy'" -o "../../dist/$bin"

cd ../../dist/
"./$bin" version

if [ "$RELEASE" ]; then
    gh release upload "$FRANKENPHP_VERSION" "$bin" --repo dunglas/frankenphp --clobber
fi

if [ "$CURRENT_REF" ]; then
    git checkout "$CURRENT_REF"
fi
