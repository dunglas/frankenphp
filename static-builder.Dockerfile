# syntax=docker/dockerfile:1
FROM golang-base

ARG FRANKENPHP_VERSION='dev'
ARG PHP_VERSION='8.3'
ARG PHP_EXTENSIONS='apcu,bcmath,bz2,calendar,ctype,curl,dba,dom,exif,fileinfo,filter,gd,iconv,intl,ldap,mbregex,mbstring,mysqli,mysqlnd,opcache,openssl,pcntl,pdo,pdo_mysql,pdo_pgsql,pdo_sqlite,pgsql,phar,posix,readline,redis,session,simplexml,sockets,sqlite3,sysvsem,tokenizer,xml,xmlreader,xmlwriter,zip,zlib'
ARG PHP_EXTENSION_LIBS='freetype,libjpeg,libavif,libwebp,libzip,bzip2'

RUN apk update; \
    apk add --no-cache \
        autoconf \
        automake \
        bash \
        binutils \
        bison \
        build-base \
        cmake \
        curl \
        file \
        flex \
        g++ \
        gcc \
        git \
        jq \
        libgcc \
        libstdc++ \
        libtool \
        linux-headers \
        m4 \
        make \
        pkgconfig \
        wget \
        xz ; \
    apk add --no-cache \
    	--repository=https://dl-cdn.alpinelinux.org/alpine/edge/main \
    	--repository=https://dl-cdn.alpinelinux.org/alpine/edge/community \
        php83 \
        php83-common \
        php83-curl \
        php83-dom \
        php83-mbstring \
        php83-openssl \
        php83-pcntl \
        php83-phar \
        php83-posix \
        php83-sodium \
        php83-tokenizer \
        php83-xml \
        php83-xmlwriter; \
    ln -sf /usr/bin/php83 /usr/bin/php

# https://getcomposer.org/doc/03-cli.md#composer-allow-superuser
ENV COMPOSER_ALLOW_SUPERUSER=1
ENV PATH="${PATH}:/root/.composer/vendor/bin"

COPY --from=composer/composer:2-bin --link /composer /usr/bin/composer

WORKDIR /static-php-cli

RUN git clone --depth=1 https://github.com/crazywhalecc/static-php-cli . && \
    composer install --no-cache --no-dev --classmap-authoritative

RUN --mount=type=secret,id=github-token GITHUB_TOKEN=$(cat /run/secrets/github-token) ./bin/spc download --with-php=$PHP_VERSION --for-extensions="$PHP_EXTENSIONS"

RUN ./bin/spc build --build-embed --enable-zts --disable-opcache-jit "$PHP_EXTENSIONS" --with-libs="$PHP_EXTENSION_LIBS"

ENV PATH="/static-php-cli/buildroot/bin:/static-php-cli/buildroot/usr/bin:$PATH"

WORKDIR /go/src/app

COPY go.mod go.sum ./
RUN go mod graph | awk '{if ($1 !~ "@") print $2}' | xargs go get

RUN mkdir caddy && cd caddy
COPY caddy/go.mod caddy/go.sum ./caddy/

RUN cd caddy && go mod graph | awk '{if ($1 !~ "@") print $2}' | xargs go get

COPY *.* ./
COPY caddy caddy
COPY C-Thread-Pool C-Thread-Pool

RUN cd caddy/frankenphp && \
    CGO_CFLAGS="$(/static-php-cli/buildroot/bin/php-config --includes | sed s#-I/#-I/static-php-cli/buildroot/#g)" \
    CGO_LDFLAGS="-DFRANKENPHP_VERSION=$FRANKENPHP_VERSION $(/static-php-cli/buildroot/bin/php-config --ldflags) -Wl,--start-group $(/static-php-cli/buildroot/bin/php-config --libs | sed -e 's/-lgcc_s//g') -Wl,--end-group" \
    LIBPHP_VERSION="$(/static-php-cli/buildroot/bin/php-config --version)" \
    go build -buildmode=pie -tags "cgo netgo osusergo static_build" -ldflags "-linkmode=external -extldflags -static-pie -s -w -X 'github.com/caddyserver/caddy/v2.CustomVersion=FrankenPHP $FRANKENPHP_VERSION PHP $LIBPHP_VERSION Caddy'" && \
    ./frankenphp version
