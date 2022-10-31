# syntax=docker/dockerfile:1.4

ARG DISTRO

#
# Base images
#
FROM php:8.2.0RC5-zts-bullseye as php-bullseye
FROM php:8.2.0RC5-zts-alpine3.16 as php-alpine316
FROM php-${DISTRO} as php

#
# Golang images
#
FROM golang:bullseye as golang-bullseye
FROM golang:alpine3.16 as golang-alpine316
FROM golang-${DISTRO} as golang

#
# php-src builder
#
# See https://github.com/docker-library/php/blob/master/Dockerfile-linux.template
# to see how php-src is build on debian and alpine
#

# Debian based
FROM php as php-debian

ENV PHPIZE_DEPS \
    autoconf \
    dpkg-dev \
    file \
    g++ \
    gcc \
    libc-dev \
    make \
    pkg-config \
    re2c

RUN apt-get update && \
    apt-get -y --no-install-recommends install \
    $PHPIZE_DEPS \
    libargon2-dev \
    libcurl4-openssl-dev \
    libonig-dev \
    libreadline-dev \
    libsodium-dev \
    libsqlite3-dev \
    libssl-dev \
    libxml2-dev \
    zlib1g-dev \
    bison \
    git

FROM php-debian as php-src-bullseye

# Alpine based
FROM php as php-alpine

ENV PHPIZE_DEPS \
    autoconf \
    dpkg-dev dpkg \
    file \
    g++ \
    gcc \
    libc-dev \
    make \
    pkgconf \
    re2c

RUN apk add --no-cache \
    $PHPIZE_DEPS \
    argon2-dev \
    coreutils \
    curl-dev \
    readline-dev \
    libsodium-dev \
    sqlite-dev \
    openssl-dev \
    libxml2-dev \
    gnu-libiconv-dev \
    linux-headers \
    oniguruma-dev \
    bison \
    git

FROM php-alpine as php-src-alpine316

#
# Clone of php-src repository
#
FROM alpine/git AS repo

RUN git clone --depth=1 --single-branch --branch=PHP-8.2 https://github.com/php/php-src.git /php-src

#
# php-src builder - actual build phase
#
FROM php-src-${DISTRO} AS php-src

RUN rm -Rf /usr/local/include/php/ /usr/local/lib/libphp.* /usr/local/lib/php/ /usr/local/php/
COPY --from=repo /php-src /php-src

WORKDIR /php-src/

# --enable-embed is only necessary to generate libphp.so, we don't use this SAPI directly
RUN ./buildconf
RUN ./configure \
        --enable-embed \
        --enable-zts \
        --disable-zend-signals \
    	# --enable-mysqlnd is included here because it's harder to compile after the fact than extensions are (since it's a plugin for several extensions, not an extension in itself)
    	--enable-mysqlnd \
     	# make sure invalid --configure-flags are fatal errors instead of just warnings
    	--enable-option-checking=fatal \
    	# https://github.com/docker-library/php/issues/439
    	--with-mhash \
    	# https://github.com/docker-library/php/issues/822
    	--with-pic \
    	# --enable-ftp is included here because ftp_ssl_connect() needs ftp to be compiled statically (see https://github.com/docker-library/php/issues/236)
    	--enable-ftp \
    	# --enable-mbstring is included here because otherwise there's no way to get pecl to use it properly (see https://github.com/docker-library/php/issues/195)
    	--enable-mbstring \
    	# https://wiki.php.net/rfc/argon2_password_hash
    	--with-password-argon2 \
    	# https://wiki.php.net/rfc/libsodium
    	--with-sodium=shared \
    	# always build against system sqlite3 (https://github.com/php/php-src/commit/6083a387a81dbbd66d6316a3a12a63f06d5f7109)
		--with-pdo-sqlite=/usr \
		--with-sqlite3=/usr \
		--with-curl \
		--with-iconv \
		--with-openssl \
		--with-readline \
		--with-zlib \
    	# https://github.com/bwoebi/phpdbg-docs/issues/1#issuecomment-163872806 ("phpdbg is primarily a CLI debugger, and is not suitable for debugging an fpm stack.")
		--disable-phpdbg \
        --with-config-file-path="$PHP_INI_DIR" \
        --with-config-file-scan-dir="$PHP_INI_DIR/conf.d"
RUN make -j$(nproc)
RUN make install
RUN rm -Rf /php-src/
RUN echo "Creating src archive for building extensions\n"
RUN tar -c -f /usr/src/php.tar.xz -J /php-src/
RUN php --version

#
# Builder
#
FROM php-src AS builder

COPY --from=golang /usr/local/go/bin/go /usr/local/bin/go
COPY --from=golang /usr/local/go /usr/local/go

WORKDIR /go/src/app

COPY go.mod go.sum ./
RUN go mod graph | awk '{if ($1 !~ "@") print $2}' | xargs go get

RUN mkdir caddy && cd caddy
COPY caddy/go.mod caddy/go.sum ./caddy/

RUN cd caddy && go mod graph | awk '{if ($1 !~ "@") print $2}' | xargs go get

COPY . .

# todo: automate this?
# see https://github.com/docker-library/php/blob/master/8.2-rc/bullseye/zts/Dockerfile#L57-L59 for php values
ENV CGO_LDFLAGS="-lssl -lcrypto -lreadline -largon2 -lcurl -lonig -lz $PHP_LDFLAGS" CGO_CFLAGS=$PHP_CFLAGS CGO_CPPFLAGS=$PHP_CPPFLAGS

RUN cd caddy/frankenphp && go build

#
# FrankenPHP image
#
FROM php AS frankenphp

WORKDIR /app

# Add PHP extension installer
# See https://github.com/mlocati/docker-php-extension-installer
COPY --from=mlocati/php-extension-installer /usr/bin/install-php-extensions /usr/local/bin/

# Copy build FrankenPHP binary
COPY --from=builder /go/src/app/caddy/frankenphp/frankenphp /usr/local/bin/frankenphp

# Copy build PHP
COPY --from=builder /usr/local/include/php/ /usr/local/include/php
COPY --from=builder /usr/local/lib/libphp.* /usr/local/lib
COPY --from=builder /usr/local/lib/php/ /usr/local/lib/php
COPY --from=builder /usr/local/php/ /usr/local/php
COPY --from=builder /usr/local/bin/ /usr/local/bin
COPY --from=builder /usr/src /usr/src

# Copy Caddy configuration
COPY caddy/frankenphp/Caddyfile /etc/Caddyfile

# Create default file to serve
RUN mkdir -p /app/public && echo '<?php phpinfo();' > /app/public/index.php

# Modify docker-php-entrypoint to start frankenphp
RUN sed -i 's/php/frankenphp run/g' /usr/local/bin/docker-php-entrypoint

CMD [ "--config", "/etc/Caddyfile" ]
