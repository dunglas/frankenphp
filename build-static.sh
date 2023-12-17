#!/bin/sh

set -o errexit

if ! type "git" > /dev/null; then
    echo "The \"git\" command must be installed."
    exit 1
fi

arch="$(uname -m)"
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
if [ "${os}" = "darwin" ]; then
    os="mac"
fi

if [ -z "${PHP_EXTENSIONS}" ]; then
    if [ "${os}" = "mac" ] && [ "${arch}" = "x86_64" ]; then
        # Temporary fix for https://github.com/crazywhalecc/static-php-cli/issues/278 (remove ldap)
        export PHP_EXTENSIONS="apcu,bcmath,bz2,calendar,ctype,curl,dba,dom,exif,fileinfo,filter,gd,iconv,intl,mbregex,mbstring,mysqli,mysqlnd,opcache,openssl,pcntl,pdo,pdo_mysql,pdo_pgsql,pdo_sqlite,pgsql,phar,posix,readline,redis,session,simplexml,sockets,sqlite3,sysvsem,tokenizer,xml,xmlreader,xmlwriter,zip,zlib"
    else
        export PHP_EXTENSIONS="apcu,bcmath,bz2,calendar,ctype,curl,dba,dom,exif,fileinfo,filter,gd,iconv,intl,ldap,mbregex,mbstring,mysqli,mysqlnd,opcache,openssl,pcntl,pdo,pdo_mysql,pdo_pgsql,pdo_sqlite,pgsql,phar,posix,readline,redis,session,simplexml,sockets,sqlite3,sysvsem,tokenizer,xml,xmlreader,xmlwriter,zip,zlib" 
    fi
fi

if [ -z "${PHP_EXTENSIONS_LIB}" ]; then
    export PHP_EXTENSION_LIBS="freetype,libjpeg,libavif,libwebp,libzip,bzip2"
fi

if [ -z "${PHP_VERSION}" ]; then
    export PHP_VERSION="8.3"
fi

if [ -z "${FRANKENPHP_VERSION}" ]; then
    FRANKENPHP_VERSION="$(git rev-parse --verify HEAD)"
    export FRANKENPHP_VERSION
elif [ -d ".git/" ]; then
    CURRENT_REF="$(git rev-parse --abbrev-ref HEAD)"
    export CURRENT_REF

    if echo "${FRANKENPHP_VERSION}" | grep -F -q "."; then
        # Tag
        git checkout "v${FRANKENPHP_VERSION}"
    else
        git checkout "${FRANKENPHP_VERSION}"
    fi
fi

bin="frankenphp-${os}-${arch}"

if [ -n "${CLEAN}" ]; then
    rm -Rf dist/
    go clean -cache
fi

# Build libphp if ncessary
if [ -f "dist/static-php-cli/buildroot/lib/libphp.a" ]; then
    cd dist/static-php-cli    
else
    mkdir -p dist/
    cd dist/

    if [ -d "static-php-cli/" ]; then
        cd static-php-cli/
        git pull
    else
        git clone --depth 1 https://github.com/crazywhalecc/static-php-cli
        cd static-php-cli/
    fi

    if type "brew" > /dev/null; then
        packages="composer"
        if ! type "go" > /dev/null; then
            packages="${packages} go"
        fi
        if [ -n "${RELEASE}" ]; then
            packages="${packages} gh"
        fi

        # shellcheck disable=SC2086
        brew install --formula --quiet ${packages}
    fi

    composer install --no-dev -a

    if [ "${os}" = "linux" ]; then
        extraOpts="--disable-opcache-jit"
    fi

    if [ -n "${DEBUG_SYMBOLS}" ]; then
        extraOpts="${extraOpts} --no-strip"
    fi

    ./bin/spc doctor
    ./bin/spc fetch --with-php="${PHP_VERSION}" --for-extensions="${PHP_EXTENSIONS}"
    # shellcheck disable=SC2086
    ./bin/spc build --debug --enable-zts --build-embed ${extraOpts} "${PHP_EXTENSIONS}" --with-libs="${PHP_EXTENSION_LIBS}"
fi

CGO_CFLAGS="-DFRANKENPHP_VERSION=${FRANKENPHP_VERSION} $(./buildroot/bin/php-config --includes | sed s#-I/#-I"${PWD}"/buildroot/#g)"
if [ -n "${DEBUG_SYMBOLS}" ]; then
    CGO_CFLAGS="-g ${CGO_CFLAGS}"
fi
export CGO_CFLAGS

if [ "${os}" = "mac" ]; then
    export CGO_LDFLAGS="-framework CoreFoundation -framework SystemConfiguration"
fi

CGO_LDFLAGS="${CGO_LDFLAGS} $(./buildroot/bin/php-config --ldflags) $(./buildroot/bin/php-config --libs)"
export CGO_LDFLAGS

LIBPHP_VERSION="$(./buildroot/bin/php-config --version)"
export LIBPHP_VERSION

cd ../..

# Embed PHP app, if any
if [ -n "${EMBED}" ] && [ -d "${EMBED}" ]; then
    tar -cf app.tar -C "${EMBED}" .
fi

if [ "${os}" = "linux" ]; then
    extraExtldflags="-Wl,-z,stack-size=0x80000"
fi

if [ -z "${DEBUG_SYMBOLS}" ]; then
    extraLdflags="-w -s"
fi

cd caddy/frankenphp/
go env
go build -buildmode=pie -tags "cgo netgo osusergo static_build" -ldflags "-linkmode=external -extldflags '-static-pie ${extraExtldflags}' ${extraLdflags} -X 'github.com/caddyserver/caddy/v2.CustomVersion=FrankenPHP ${FRANKENPHP_VERSION} PHP ${LIBPHP_VERSION} Caddy'" -o "../../dist/${bin}"
cd ../..

if [ -d "${EMBED}" ]; then
    truncate -s 0 app.tar
fi

"dist/${bin}" version

if [ -n "${RELEASE}" ]; then
    gh release upload "v${FRANKENPHP_VERSION}" "dist/${bin}" --repo dunglas/frankenphp --clobber
fi

if [ -n "${CURRENT_REF}" ]; then
    git checkout "${CURRENT_REF}"
fi
