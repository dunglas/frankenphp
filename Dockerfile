FROM php:8.2.0RC4-zts-bullseye AS php-base

# Note that this image is based on the official PHP image, once 8.3 is released, this stage can likely be removed

RUN rm -Rf /usr/local/include/php/ /usr/local/lib/libphp.* /usr/local/lib/php/ /usr/local/php/

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
    git \
    && \
    apt-get clean

RUN git clone --depth=1 --single-branch --branch=frankenphp-8.2 https://github.com/dunglas/php-src.git && \
    cd php-src && \
    # --enable-embed is only necessary to generate libphp.so, we don't use this SAPI directly
    ./buildconf && \
    ./configure \
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
        --with-config-file-scan-dir="$PHP_INI_DIR/conf.d" && \
    make -j$(nproc) && \
    make install && \
    rm -Rf php-src/ && \
    echo "Creating src archive for building extensions\n" && \
    tar -c -f /usr/src/php.tar.xz -J /php-src/ && \
    ldconfig && \
    php --version

FROM php-base AS builder

COPY --from=golang:bullseye /usr/local/go/bin/go /usr/local/bin/go
COPY --from=golang:bullseye /usr/local/go /usr/local/go

WORKDIR /go/src/app

COPY go.mod go.sum ./
RUN go mod graph | awk '{if ($1 !~ "@") print $2}' | xargs go get

RUN mkdir caddy && cd caddy
COPY caddy/go.mod caddy/go.sum ./caddy/

RUN cd caddy && go mod graph | awk '{if ($1 !~ "@") print $2}' | xargs go get

COPY *.* .
COPY caddy caddy
COPY C-Thread-Pool C-Thread-Pool
COPY internal internal
COPY testdata testdata

# todo: automate this?
# see https://github.com/docker-library/php/blob/master/8.2-rc/bullseye/zts/Dockerfile#L57-L59 for php values
ENV CGO_LDFLAGS="-lssl -lcrypto -lreadline -largon2 -lcurl -lonig -lz $PHP_LDFLAGS" CGO_CFLAGS=$PHP_CFLAGS CGO_CPPFLAGS=$PHP_CPPFLAGS

RUN cd caddy/frankenphp && \
    go build && \
    cp frankenphp /usr/local/bin && \
    cp /go/src/app/caddy/frankenphp/Caddyfile /etc/Caddyfile

ENTRYPOINT ["/bin/bash","-c"]

FROM php:8.2.0RC4-zts-bullseye AS final

COPY --from=mlocati/php-extension-installer /usr/bin/install-php-extensions /usr/local/bin/

WORKDIR /app

RUN mkdir -p /app/public
RUN echo '<?php phpinfo();' > /app/public/index.php

COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp
COPY --from=builder /etc/Caddyfile /etc/Caddyfile

COPY --from=php-base /usr/local/include/php/ /usr/local/include/php
COPY --from=php-base /usr/local/lib/libphp.* /usr/local/lib
COPY --from=php-base /usr/local/lib/php/ /usr/local/lib/php
COPY --from=php-base /usr/local/php/ /usr/local/php
COPY --from=php-base /usr/local/bin/ /usr/local/bin
COPY --from=php-base /usr/src /usr/src

RUN sed -i 's/php/frankenphp run/g' /usr/local/bin/docker-php-entrypoint

CMD [ "--config", "/etc/Caddyfile" ]
