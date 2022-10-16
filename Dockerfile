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
    libbz2-dev \
    libenchant-2-dev \
    libpng-dev \
    libgmp-dev \
    libavif-dev \
    libwebp-dev \
    libjpeg-dev \
    libfreetype-dev \
    libldap-dev \
    libpq-dev \
    libxslt1-dev \
    libzip-dev \
    libicu-dev \
    bison \
    && \
    apt-get clean

RUN git clone --depth=1 --single-branch --branch=frankenphp-8.2 https://github.com/dunglas/php-src.git

RUN cd php-src && ./buildconf

RUN cd php-src && \
    `# --enable-embed is only necessary to generate libphp.so, we don't use this SAPI directly` \
    ./configure \
        --disable-fpm \
        --disable-zend-signals \
        --enable-fileinfo \
        --enable-dom \
        --enable-filter \
        --with-libxml \
        --with-iconv \
        --enable-bcmath \
        --enable-calendar \
        --enable-embed=static \
        --enable-exif \
        --enable-ftp \
        --enable-gd \
        --enable-intl \
        --enable-mbstring \
        --enable-pcntl \
        --enable-shmop \
        --enable-sigchild \
        --enable-soap \
        --enable-sockets \
        --enable-sysvmsg \
        --enable-sysvsem \
        --enable-sysvshm \
        --enable-zts \
        --with-avif \
        --with-bz2 \
        --with-curl \
        --with-ffi \
        --with-freetype \
        --with-gettext \
        --with-gmp \
        --with-jpeg \
        --with-ldap \
        --with-mysqli \
        --with-openssl \
        --with-password-argon2 \
        --with-pdo-mysql \
        --with-pdo-pgsql \
        --with-pdo-sqlite \
        --with-sodium \
        --with-webp \
        --with-xsl \
        --with-zip \
        --with-zlib \
        --with-config-file-scan-dir=/usr/local/lib/php/ \
    && \
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
