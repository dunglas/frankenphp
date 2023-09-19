# syntax=docker/dockerfile:1
FROM golang:1.21-alpine

ENV CFLAGS="-ggdb3"
ENV PHPIZE_DEPS \
    autoconf \
    dpkg-dev \
    file \
    g++ \
    gcc \
    libc-dev \
    make \
    pkgconfig \
    re2c

RUN apk update && \
    apk add --no-cache \
    $PHPIZE_DEPS \
    argon2-dev \
    curl-dev \
    oniguruma-dev \
    readline-dev \
    libsodium-dev \
    sqlite-dev \
    openssl-dev \
    libxml2-dev \
    zlib-dev \
    bison \
    nss-tools \
    # Dev tools \
    git \
    clang \
    llvm \
    gdb \
    valgrind \
    neovim \
    zsh \
    libtool && \
    echo 'set auto-load safe-path /' > /root/.gdbinit && \
    rm -rf /var/cache/apk/*

RUN git clone --branch=PHP-8.2 https://github.com/php/php-src.git && \
    cd php-src && \
    # --enable-embed is only necessary to generate libphp.so, we don't use this SAPI directly
    ./buildconf --force && \
    ./configure \
        --enable-embed \
        --enable-zts \
        --disable-zend-signals \
        --enable-zend-max-execution-timers \
        --enable-debug && \
    make -j$(nproc) && \
    make install && \
    ldconfig /etc/ld.so.conf.d && \
    cp php.ini-development /usr/local/lib/php.ini && \
    echo -e "zend_extension=opcache.so\nopcache.enable=1" >> /usr/local/lib/php.ini &&\
    php --version

WORKDIR /go/src/app

COPY . .

RUN ls && cd caddy/frankenphp && \
    go build

CMD [ "zsh" ]
