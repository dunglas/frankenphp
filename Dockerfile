# syntax=docker/dockerfile:1
FROM php-base AS builder

ARG FRANKENPHP_VERSION='dev'

ENV PATH /usr/local/go/bin:$PATH
# todo: automate this?
# see https://github.com/docker-library/php/blob/master/8.2/bookworm/zts/Dockerfile#L57-L59 for PHP values
ENV CGO_LDFLAGS="-lssl -lcrypto -lreadline -largon2 -lcurl -lonig -lz $PHP_LDFLAGS" CGO_CFLAGS=$PHP_CFLAGS CGO_CPPFLAGS=$PHP_CPPFLAGS

ENTRYPOINT ["/bin/bash","-c"]

# This is required to link the FrankenPHP binary to the PHP binary
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

COPY --from=golang-base /usr/local/go/bin/go /usr/local/go/bin/go
COPY --from=golang-base /usr/local/go /usr/local/go

RUN --mount=type=cache,target=/go/pkg/mod/ \
	--mount=type=bind,source=go.sum,target=go.sum \
	--mount=type=bind,source=go.mod,target=go.mod \
    go mod graph | awk '{if ($1 !~ "@") print $2}' | xargs go get

RUN mkdir caddy
RUN --mount=type=cache,target=/go/pkg/mod/ \
	--mount=type=bind,source=caddy/go.sum,target=caddy/go.sum \
	--mount=type=bind,source=caddy/go.mod,target=caddy/go.mod \
    cd caddy && go mod graph | awk '{if ($1 !~ "@") print $2}' | xargs go get

RUN --mount=type=bind,target=. \
    cd caddy/frankenphp && \
    go build -ldflags "-X 'github.com/caddyserver/caddy/v2.CustomVersion=FrankenPHP $FRANKENPHP_VERSION Caddy'" && \
    cp frankenphp /usr/local/bin && \
    cp /go/src/app/caddy/frankenphp/Caddyfile /etc/Caddyfile


FROM php-base AS runner

ENV GODEBUG=cgocheck=0

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
