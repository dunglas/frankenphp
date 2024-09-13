# syntax=docker/dockerfile:1
#checkov:skip=CKV_DOCKER_2
#checkov:skip=CKV_DOCKER_3
FROM golang:1.22

ENV CFLAGS="-ggdb3"
ENV PHPIZE_DEPS="\
	autoconf \
	dpkg-dev \
	file \
	g++ \
	gcc \
	libc-dev \
	make \
	pkg-config \
	re2c"

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# hadolint ignore=DL3009
RUN apt-get update && \
	apt-get -y --no-install-recommends install \
	$PHPIZE_DEPS \
	libargon2-dev \
	libbrotli-dev \
	libcurl4-openssl-dev \
	libonig-dev \
	libreadline-dev \
	libsodium-dev \
	libsqlite3-dev \
	libssl-dev \
	libxml2-dev \
	zlib1g-dev \
	bison \
	libnss3-tools \
	# Dev tools \
	git \
	clang \
	llvm \
	gdb \
	valgrind \
	neovim \
	zsh \
    meson \
	libtool-bin && \
	echo 'set auto-load safe-path /' > /root/.gdbinit && \
	echo '* soft core unlimited' >> /etc/security/limits.conf \
	&& \
	apt-get clean 

WORKDIR /usr/local/src/php
RUN git clone --branch=PHP-8.3 https://github.com/php/php-src.git . && \
	# --enable-embed is only necessary to generate libphp.so, we don't use this SAPI directly
	./buildconf --force && \
	./configure \
		--enable-embed \
		--enable-zts \
		--disable-zend-signals \
		--enable-zend-max-execution-timers \
		--enable-debug && \
	make -j"$(nproc)" && \
	make install && \
	ldconfig && \
	cp php.ini-development /usr/local/lib/php.ini && \
	echo "zend_extension=opcache.so" >> /usr/local/lib/php.ini && \
	echo "opcache.enable=1" >> /usr/local/lib/php.ini && \
	php --version

# install edant/watcher (necessary for file watching)
WORKDIR /usr/local/src/watcher
RUN git clone --branch=next https://github.com/e-dant/watcher .
WORKDIR /usr/local/src/watcher/watcher-c
RUN meson build .. && \
	meson compile -C build && \
	cp -r build/watcher-c/libwatcher-c* /usr/local/lib/ && \
	ldconfig
# TODO: alternatively edant/watcher install with clang++ or cmake? (will create a libwatcher.so):
# RUN clang++ -o libwatcher.so ./src/watcher-c.cpp -I ./include -I ../include -std=c++17 -O3 -Wall -Wextra -fPIC -shared && \
#     cp libwatcher.so /usr/local/lib/libwatcher.so && \
#     ldconfig

WORKDIR /go/src/app
COPY . .

WORKDIR /go/src/app/caddy/frankenphp
RUN go build -buildvcs=false

WORKDIR /go/src/app
CMD [ "zsh" ]
