ARG DISTRO
ARG PHP_VERSION

#
# Base images
#
FROM php:${PHP_VERSION}-zts-bullseye as php-bullseye
FROM php:${PHP_VERSION}-zts-buster as php-buster
FROM php:${PHP_VERSION}-zts-alpine3.15 as php-alpine315
FROM php:${PHP_VERSION}-zts-alpine3.16 as php-alpine316
FROM php-${DISTRO} as php

#
# Golang images
#
FROM golang:bullseye as golang-bullseye
FROM golang:buster as golang-buster
FROM golang:alpine3.15 as golang-alpine315
FROM golang:alpine3.16 as golang-alpine316
FROM golang-${DISTRO} as golang

#
# php-src builder
#
FROM php AS php-src

#
# Builder
#
FROM php as builder

COPY --from=golang /usr/local/go/bin/go /usr/local/bin/go
COPY --from=golang /usr/local/go /usr/local/go

### DEBUG STARTS ###
WORKDIR /test_copying
COPY . .
RUN ls -lah
### DEBUG ENDS   ###

#
# FrankenPHP image
#
FROM php AS frankenphp

COPY --from=mlocati/php-extension-installer /usr/bin/install-php-extensions /usr/local/bin/

COPY --from=builder /usr/local/go /usr/local/go

RUN mkdir -p /app/public && echo '<?php phpinfo();' > /app/public/index.php

WORKDIR /app

### DEBUG STARTS ###
RUN cat /etc/os-release
### DEBUG ENDS   ###

# Modify docker-php-entrypoint to start frankenphp
RUN sed -i 's/php/frankenphp run/g' /usr/local/bin/docker-php-entrypoint

CMD [ "--config", "/etc/Caddyfile" ]
