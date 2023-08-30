# syntax=docker/dockerfile:1
FROM golang-base

ARG FRANKENPHP_VERSION='dev'
ARG PHP_VERSION='8.2'
ARG PHP_EXTENSIONS='bcmath,calendar,ctype,curl,dba,dom,exif,filter,fileinfo,gd,iconv,mbstring,mbregex,mysqli,mysqlnd,openssl,pcntl,pdo,pdo_mysql,pdo_pgsql,pdo_sqlite,pgsql,phar,posix,readline,redis,session,simplexml,sockets,sqlite3,tokenizer,xml,xmlreader,xmlwriter,zip,zlib,apcu'

RUN apk update; \
    apk add --no-cache \
        autoconf \
        automake \
        bash \
        binutils \
        bison \
        build-base \
        cmake \
        composer \
        curl \
        file \
        flex \
        g++ \
        gcc \
        git \
        jq \
        libgcc \
        libstdc++ \
        linux-headers \
        m4 \
        make \
        php81 \
        php81-common \
        php81-pcntl \
        php81-phar \
        php81-posix \
        php81-tokenizer \
        php81-xml \
        pkgconfig \
        wget \
        xz

WORKDIR /static-php-cli

RUN git clone --depth=1 https://github.com/dunglas/static-php-cli.git --branch=feat/embed . && \
    composer install --no-cache --no-dev --classmap-authoritative

RUN --mount=type=secret,id=github-token GITHUB_TOKEN=$(cat /run/secrets/github-token) ./bin/spc download --with-php=$PHP_VERSION --all

RUN ./bin/spc build --build-embed --enable-zts --debug "$PHP_EXTENSIONS"

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
    CGO_LDFLAGS="$(/static-php-cli/buildroot/bin/php-config --ldflags) $(/static-php-cli/buildroot/bin/php-config --libs | sed -e 's/-lgcc_s//g')" \
    go build -buildmode=pie -tags "cgo netgo osusergo static_build" -ldflags "-linkmode=external -extldflags -static-pie -s -w -X 'github.com/caddyserver/caddy/v2.CustomVersion=FrankenPHP $FRANKENPHP_VERSION Caddy'"
