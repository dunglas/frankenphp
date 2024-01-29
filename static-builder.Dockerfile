# syntax=docker/dockerfile:1
FROM golang-base

ARG FRANKENPHP_VERSION=''
ENV FRANKENPHP_VERSION=${FRANKENPHP_VERSION}

ARG PHP_VERSION=''
ENV PHP_VERSION=${PHP_VERSION}

ARG PHP_EXTENSIONS=''
ENV PHP_EXTENSIONS=${PHP_EXTENSIONS}

ARG PHP_EXTENSION_LIBS=''
ENV PHP_EXTENSION_LIBS=${PHP_EXTENSION_LIBS}

ARG CLEAN=''
ARG EMBED=''
SHELL ["/bin/ash", "-eo", "pipefail", "-c"]

LABEL org.opencontainers.image.title=FrankenPHP
LABEL org.opencontainers.image.description="The modern PHP app server"
LABEL org.opencontainers.image.url=https://frankenphp.dev
LABEL org.opencontainers.image.source=https://github.com/dunglas/frankenphp
LABEL org.opencontainers.image.licenses=MIT
LABEL org.opencontainers.image.vendor="Kévin Dunglas"

RUN apk update; \
	apk add --no-cache \
		autoconf \
		automake \
		bash \
		binutils \
		bison \
		build-base \
		cmake \
		curl \
		file \
		flex \
		g++ \
		gcc \
		git \
		jq \
		libgcc \
		libstdc++ \
		libtool \
		linux-headers \
		m4 \
		make \
		pkgconfig \
		wget \
		xz ; \
	apk add --no-cache \
		--repository=https://dl-cdn.alpinelinux.org/alpine/edge/main \
		--repository=https://dl-cdn.alpinelinux.org/alpine/edge/community \
		php83 \
		php83-common \
		php83-ctype \
		php83-curl \
		php83-dom \
		php83-mbstring \
		php83-openssl \
		php83-pcntl \
		php83-phar \
		php83-posix \
		php83-session \
		php83-sodium \
		php83-tokenizer \
		php83-xml \
		php83-xmlwriter; \
	ln -sf /usr/bin/php83 /usr/bin/php

# https://getcomposer.org/doc/03-cli.md#composer-allow-superuser
ENV COMPOSER_ALLOW_SUPERUSER=1
ENV PATH="${PATH}:/root/.composer/vendor/bin"

COPY --from=composer/composer:2-bin --link /composer /usr/bin/composer

WORKDIR /go/src/app
COPY go.mod go.sum ./
RUN go mod graph | awk '{if ($1 !~ "@") print $2}' | xargs go get

WORKDIR /go/src/app/caddy
COPY caddy/go.mod caddy/go.sum ./
RUN go mod graph | awk '{if ($1 !~ "@") print $2}' | xargs go get

WORKDIR /go/src/app
COPY *.* ./
COPY caddy caddy
COPY C-Thread-Pool C-Thread-Pool

RUN --mount=type=secret,id=github-token GITHUB_TOKEN=$(cat /run/secrets/github-token) ./build-static.sh
