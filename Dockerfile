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
    # Dev tools \
    git \
    gdb \
    valgrind \
    neovim && \
    echo 'set auto-load safe-path /' > /root/.gdbinit && \
    echo '* soft core unlimited' >> /etc/security/limits.conf \
    && \
    apt-get clean 

RUN git clone https://github.com/dunglas/php-src.git && \
    cd php-src && \
    git checkout frankenphp-8.2 && \
    # --enable-embed is only necessary to generate libphp.so, we don't use this SAPI directly
    ./buildconf && \
    ./configure --enable-embed=static --enable-zts --disable-zend-signals --enable-static --enable-debug && \
    make -j6 && \
    make install && \
    #rm -Rf php-src/ && \
    ldconfig && \
    php --version

RUN echo "zend_extension=opcache.so\nopcache.enable=1" > /usr/local/lib/php.ini

WORKDIR /go/src/app

COPY . .

RUN go get -d -v ./...
