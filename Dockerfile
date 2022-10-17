FROM php:8.2.0RC4-zts-bullseye AS php-base

COPY --from=mlocati/php-extension-installer /usr/bin/install-php-extensions /usr/local/bin/
ADD https://github.com/dunglas/php-src/archive/refs/heads/frankenphp-8.2.zip /frankenphp-8.2.zip

RUN apt update && \
    apt install -y unzip && \
    unzip frankenphp-8.2.zip && \
    tar -c -f /usr/src/php.tar.xz -J /php-src-frankenphp-8.2/ && \
    rm -rf /frankenphp-8.2.zip /php-src-frankenphp-8.2

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
    #export CFLAGS="-DNO_SIGPROF" && \
    # --enable-embed is only necessary to generate libphp.so, we don't use this SAPI directly
    ./buildconf && \
    ./configure \
        --enable-embed=static \
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
    ldconfig && \
    install-php-extensions opcache && \
    php --version

#RUN echo "zend_extension=opcache.so\nopcache.enable=1" > /usr/local/lib/php.ini

FROM golang AS server

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
    && \
    apt-get clean

RUN git clone --depth=1 --single-branch --branch=frankenphp-8.2 https://github.com/dunglas/php-src.git && \
    cd php-src && \
    #export CFLAGS="-DNO_SIGPROF" && \
    # --enable-embed is only necessary to generate libphp.so, we don't use this SAPI directly
    ./buildconf && \
    ./configure \
        --enable-embed=static \
        --enable-zts \
        --disable-zend-signals \
        --with-config-file-path="$PHP_INI_DIR" \
        --with-config-file-scan-dir="$PHP_INI_DIR/conf.d" && \
    make -j$(nproc) && \
    make install && \
    rm -Rf php-src/ && \
    ldconfig && \
    php --version

WORKDIR /go/src/app

COPY go.mod go.sum ./
RUN go get -v ./...

RUN mkdir caddy && cd caddy
COPY go.mod go.sum ./

RUN go get -v ./... && \
    cd ..

COPY . .

RUN cd caddy/frankenphp && \
    go build && \
    cp frankenphp /usr/local/bin && \
    cp /go/src/app/caddy/frankenphp/Caddyfile /etc/Caddyfile && \
    rm -Rf /go

CMD [ "frankenphp", "run", "--config", "/etc/Caddyfile" ]

FROM php-base AS final

WORKDIR /app

RUN mkdir -p /app/public
RUN echo '<?php phpinfo();' > /app/public/index.php

COPY --from=server /usr/local/bin/frankenphp /usr/local/bin/frankenphp
COPY --from=server /etc/Caddyfile /etc/Caddyfile

RUN install-php-extensions pdo_mysql

ENTRYPOINT [ "frankenphp", "run", "--config", "/etc/Caddyfile" ]