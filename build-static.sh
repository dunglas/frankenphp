#!/bin/sh

set -o errexit
set -x

if ! type "git" > /dev/null; then
    echo "The \"git\" command must be installed."
    exit 1
fi

arch="$(uname -m)"
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
md5binary="md5sum"
if [ "${os}" = "darwin" ]; then
    os="mac"
    md5binary="md5 -q"
fi

if [ "${os}" = "linux" ] && ! type "cmake" > /dev/null; then
    echo "The \"cmake\" command must be installed."
    exit 1
fi

if [ -z "${PHP_EXTENSIONS}" ]; then
    export PHP_EXTENSIONS="apcu,bcmath,bz2,calendar,ctype,curl,dba,dom,exif,fileinfo,filter,gd,iconv,igbinary,intl,ldap,mbregex,mbstring,mysqli,mysqlnd,opcache,openssl,pcntl,pdo,pdo_mysql,pdo_pgsql,pdo_sqlite,pgsql,phar,posix,readline,redis,session,simplexml,sockets,sodium,sqlite3,sysvsem,tokenizer,xml,xmlreader,xmlwriter,zip,zlib"
fi

if [ -z "${PHP_EXTENSION_LIBS}" ]; then
    export PHP_EXTENSION_LIBS="bzip2,freetype,libavif,libjpeg,liblz4,libwebp,libzip"
fi

# The Brotli library must always be built as it is required by http://github.com/dunglas/caddy-cbrotli
if ! echo "${PHP_EXTENSION_LIBS}" | grep -q "\bbrotli\b"; then
    export PHP_EXTENSION_LIBS="${PHP_EXTENSION_LIBS},brotli"
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

        # Trim "v" prefix if any
        FRANKENPHP_VERSION=${FRANKENPHP_VERSION#v}
        export FRANKENPHP_VERSION

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

# Build libphp if necessary
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
        if ! type "composer" > /dev/null; then
            packages="composer"
        fi
        if ! type "go" > /dev/null; then
            packages="${packages} go"
        fi
        if [ -n "${RELEASE}" ] && ! type "gh" > /dev/null; then
            packages="${packages} gh"
        fi

        if [ -n "${packages}" ]; then
            # shellcheck disable=SC2086
            brew install --formula --quiet ${packages}
        fi
    fi

    composer install --no-dev -a

    if [ "${os}" = "linux" ]; then
        extraOpts="--disable-opcache-jit"
    fi

    if [ -n "${DEBUG_SYMBOLS}" ]; then
        extraOpts="${extraOpts} --no-strip"
    fi

    ./bin/spc doctor --auto-fix
    ./bin/spc download --with-php="${PHP_VERSION}" --for-extensions="${PHP_EXTENSIONS}" --for-libs="${PHP_EXTENSION_LIBS}" --ignore-cache-sources=php-src
    # shellcheck disable=SC2086
    ./bin/spc build --debug --enable-zts --build-embed ${extraOpts} "${PHP_EXTENSIONS}" --with-libs="${PHP_EXTENSION_LIBS}"
fi

CGO_CFLAGS="-DFRANKENPHP_VERSION=${FRANKENPHP_VERSION} -I${PWD}/buildroot/include/ $(./buildroot/bin/php-config --includes | sed s#-I/#-I"${PWD}"/buildroot/#g)"
if [ -n "${DEBUG_SYMBOLS}" ]; then
    CGO_CFLAGS="-g ${CGO_CFLAGS}"
fi
export CGO_CFLAGS

if [ "${os}" = "mac" ]; then
    export CGO_LDFLAGS="-framework CoreFoundation -framework SystemConfiguration"
fi

CGO_LDFLAGS="${CGO_LDFLAGS} ${PWD}/buildroot/lib/libbrotlicommon.a ${PWD}/buildroot/lib/libbrotlienc.a ${PWD}/buildroot/lib/libbrotlidec.a $(./buildroot/bin/php-config --ldflags || true) $(./buildroot/bin/php-config --libs || true)"
export CGO_LDFLAGS

LIBPHP_VERSION="$(./buildroot/bin/php-config --version)"
export LIBPHP_VERSION

cd ../

if [ "${os}" = "linux" ]; then
    # Replace mallocng by mimalloc
    # The default musl allocator is slow, especially when used by multi-threaded apps,
    # and triggers weird bugs
    if [ ! -f "mimalloc/out/release/libmimalloc.a" ]; then
        if [ -d "static-php-cli/" ]; then
            cd mimalloc/
            git pull
        else
            git clone https://github.com/microsoft/mimalloc.git
            cd mimalloc/
        fi

        git checkout "$(git describe --tags "$(git rev-list --tags --max-count=1 || true)" || true)"
        mkdir -p out/release/
        cd out/release/
        if [ -n "${DEBUG_SYMBOLS}" ]; then
            cmake -DCMAKE_BUILD_TYPE=Debug ../..
        else
            cmake ../..
        fi
        make -j"$(nproc || true)"

        cd ../../../
    fi

    # Also increase the default stack size to prevents issues with code including many files such as Symfony containers
    extraExtldflags="-Wl,--push-state,--whole-archive,${PWD}/mimalloc/out/release/libmimalloc.a,--pop-state,-z,stack-size=0x80000"
fi

if [ -z "${DEBUG_SYMBOLS}" ]; then
    extraLdflags="-w -s"
fi

cd ../

# Embed PHP app, if any
if [ -n "${EMBED}" ] && [ -d "${EMBED}" ]; then
    tar -cf app.tar -C "${EMBED}" .
    ${md5binary} app.tar > app_checksum.txt
fi

cd caddy/frankenphp/
go env
go build -buildmode=pie -tags "cgo netgo osusergo static_build" -ldflags "-linkmode=external -extldflags '-static-pie ${extraExtldflags}' ${extraLdflags} -X 'github.com/caddyserver/caddy/v2.CustomVersion=FrankenPHP ${FRANKENPHP_VERSION} PHP ${LIBPHP_VERSION} Caddy'" -o "../../dist/${bin}"
cd ../..

if [ -d "${EMBED}" ]; then
    truncate -s 0 app.tar
    truncate -s 0 app_checksum.txt
fi

if type "upx" > /dev/null && [ -z "${DEBUG_SYMBOLS}" ] && [ -z "${NO_COMPRESS}" ]; then
    upx --best "dist/${bin}"
fi

"dist/${bin}" version

if [ -n "${RELEASE}" ]; then
    gh release upload "v${FRANKENPHP_VERSION}" "dist/${bin}" --repo dunglas/frankenphp --clobber
fi

if [ -n "${CURRENT_REF}" ]; then
    git checkout "${CURRENT_REF}"
fi
