FROM php:8.2.0RC5-zts-bullseye AS php-base

# rebuild PHP until https://github.com/docker-library/php/pull/1331 is merged
RUN set -eux; \
	\
	savedAptMark="$(apt-mark showmanual)"; \
	apt-get update; \
	apt-get install -y --no-install-recommends \
		libargon2-dev \
		libcurl4-openssl-dev \
		libonig-dev \
		libreadline-dev \
		libsodium-dev \
		libsqlite3-dev \
		libssl-dev \
		libxml2-dev \
		zlib1g-dev \
	; \
	\
	export \
		CFLAGS="$PHP_CFLAGS" \
		CPPFLAGS="$PHP_CPPFLAGS" \
		LDFLAGS="$PHP_LDFLAGS" \
	; \
	docker-php-source extract; \
	cd /usr/src/php; \
	gnuArch="$(dpkg-architecture --query DEB_BUILD_GNU_TYPE)"; \
	debMultiarch="$(dpkg-architecture --query DEB_BUILD_MULTIARCH)"; \
	./configure \
		--build="$gnuArch" \
		--with-config-file-path="$PHP_INI_DIR" \
		--with-config-file-scan-dir="$PHP_INI_DIR/conf.d" \
		\
# make sure invalid --configure-flags are fatal errors instead of just warnings
		--enable-option-checking=fatal \
		\
# https://github.com/docker-library/php/issues/439
		--with-mhash \
		\
# https://github.com/docker-library/php/issues/822
		--with-pic \
		\
# --enable-ftp is included here because ftp_ssl_connect() needs ftp to be compiled statically (see https://github.com/docker-library/php/issues/236)
		--enable-ftp \
# --enable-mbstring is included here because otherwise there's no way to get pecl to use it properly (see https://github.com/docker-library/php/issues/195)
		--enable-mbstring \
# --enable-mysqlnd is included here because it's harder to compile after the fact than extensions are (since it's a plugin for several extensions, not an extension in itself)
		--enable-mysqlnd \
# https://wiki.php.net/rfc/argon2_password_hash
		--with-password-argon2 \
# https://wiki.php.net/rfc/libsodium
		--with-sodium=shared \
# always build against system sqlite3 (https://github.com/php/php-src/commit/6083a387a81dbbd66d6316a3a12a63f06d5f7109)
		--with-pdo-sqlite=/usr \
		--with-sqlite3=/usr \
		\
		--with-curl \
		--with-iconv \
		--with-openssl \
		--with-readline \
		--with-zlib \
		\
# https://github.com/bwoebi/phpdbg-docs/issues/1#issuecomment-163872806 ("phpdbg is primarily a CLI debugger, and is not suitable for debugging an fpm stack.")
		--disable-phpdbg \
		\
# in PHP 7.4+, the pecl/pear installers are officially deprecated (requiring an explicit "--with-pear")
		--with-pear \
		\
# bundled pcre does not support JIT on s390x
# https://manpages.debian.org/bullseye/libpcre3-dev/pcrejit.3.en.html#AVAILABILITY_OF_JIT_SUPPORT
		$(test "$gnuArch" = 's390x-linux-gnu' && echo '--without-pcre-jit') \
		--with-libdir="lib/$debMultiarch" \
		\
		--disable-cgi \
		\
# https://github.com/docker-library/php/pull/939#issuecomment-730501748
		--enable-embed \
		\
		--enable-zts \
# https://externals.io/message/118859
        --disable-zend-signals \
	; \
	make -j "$(nproc)"; \
	find -type f -name '*.a' -delete; \
	make install; \
	find \
		/usr/local \
		-type f \
		-perm '/0111' \
		-exec sh -euxc ' \
			strip --strip-all "$@" || : \
		' -- '{}' + \
	; \
	make clean; \
	\
# https://github.com/docker-library/php/issues/692 (copy default example "php.ini" files somewhere easily discoverable)
	cp -v php.ini-* "$PHP_INI_DIR/"; \
	\
	cd /; \
	docker-php-source delete; \
	\
# reset apt-mark's "manual" list so that "purge --auto-remove" will remove all build dependencies
	apt-mark auto '.*' > /dev/null; \
	[ -z "$savedAptMark" ] || apt-mark manual $savedAptMark; \
	find /usr/local -type f -executable -exec ldd '{}' ';' \
		| awk '/=>/ { print $(NF-1) }' \
		| sort -u \
		| xargs -r dpkg-query --search \
		| cut -d: -f1 \
		| sort -u \
		| xargs -r apt-mark manual \
	; \
	apt-get purge -y --auto-remove -o APT::AutoRemove::RecommendsImportant=false; \
	rm -rf /var/lib/apt/lists/*; \
	\
# update pecl channel definitions https://github.com/docker-library/php/issues/443
	pecl update-channels; \
	rm -rf /tmp/pear ~/.pearrc; \
	\
# smoke test
	php --version

FROM php-base AS builder

COPY --from=golang:bullseye /usr/local/go/bin/go /usr/local/bin/go
COPY --from=golang:bullseye /usr/local/go /usr/local/go

# This is required to link the frankenPHP binary to the PHP binary
RUN apt-get update && \
    apt-get -y --no-install-recommends install \
        libargon2-dev \
        libcurl4-openssl-dev \
        libonig-dev \
        libreadline-dev \
        libsodium-dev \
        libsqlite3-dev \
        libssl-dev \
        libxml2-dev \
        zlib1g-dev \
        && \
        apt-get clean

WORKDIR /go/src/app

COPY go.mod go.sum ./
RUN go mod graph | awk '{if ($1 !~ "@") print $2}' | xargs go get

RUN mkdir caddy && cd caddy
COPY caddy/go.mod caddy/go.sum ./caddy/

RUN cd caddy && go mod graph | awk '{if ($1 !~ "@") print $2}' | xargs go get

COPY *.* ./
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

FROM php-base AS final

COPY --from=mlocati/php-extension-installer /usr/bin/install-php-extensions /usr/local/bin/

WORKDIR /app

RUN mkdir -p /app/public
RUN echo '<?php phpinfo();' > /app/public/index.php

COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp
COPY --from=builder /etc/Caddyfile /etc/Caddyfile

RUN sed -i 's/php/frankenphp run/g' /usr/local/bin/docker-php-entrypoint

CMD [ "--config", "/etc/Caddyfile" ]
