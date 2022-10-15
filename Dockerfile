FROM golang

ARG LIBICONV_VERSION=1.17
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
        --disable-zend-signals && \
    make -j$(nproc) && \
    make install && \
    rm -Rf php-src/ && \
    ldconfig && \
    php --version

RUN echo "zend_extension=opcache.so\nopcache.enable=1" > /usr/local/lib/php.ini

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

WORKDIR /app

RUN mkdir -p /app/public
RUN echo '<?php phpinfo();' > /app/public/index.php

CMD [ "frankenphp", "run", "--config", "/etc/Caddyfile" ]
