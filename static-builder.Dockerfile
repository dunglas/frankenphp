# syntax=docker/dockerfile:1
#checkov:skip=CKV_DOCKER_2
#checkov:skip=CKV_DOCKER_3
#checkov:skip=CKV_DOCKER_7
FROM golang-base

ARG TARGETARCH

ARG FRANKENPHP_VERSION=''
ENV FRANKENPHP_VERSION=${FRANKENPHP_VERSION}

ARG PHP_VERSION=''
ENV PHP_VERSION=${PHP_VERSION}

ARG PHP_EXTENSIONS=''
ARG PHP_EXTENSION_LIBS=''
ARG XCADDY_ARGS=''
ARG CLEAN=''
ARG EMBED=''
ARG DEBUG_SYMBOLS=''
ARG MIMALLOC=''
ARG NO_COMPRESS=''

SHELL ["/bin/ash", "-eo", "pipefail", "-c"]

LABEL org.opencontainers.image.title=FrankenPHP
LABEL org.opencontainers.image.description="The modern PHP app server"
LABEL org.opencontainers.image.url=https://frankenphp.dev
LABEL org.opencontainers.image.source=https://github.com/dunglas/frankenphp
LABEL org.opencontainers.image.licenses=MIT
LABEL org.opencontainers.image.vendor="Kévin Dunglas"

RUN apk update; \
	apk add --no-cache \
		alpine-sdk \
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
		php83-xmlwriter \
		upx \
		wget \
		xz ; \
	ln -sf /usr/bin/php83 /usr/bin/php

# FIXME: temporary workaround for https://github.com/golang/go/issues/68285
WORKDIR /
RUN git clone https://go.googlesource.com/go goroot
WORKDIR /goroot
# Revert https://github.com/golang/go/commit/3560cf0afb3c29300a6c88ccd98256949ca7a6f6 to prevent the crash with musl
RUN git config --global user.email "build@example.com" && \
	git config --global user.name "Build" && \
	git checkout "$(go env GOVERSION)" && \
	git revert 3560cf0afb3c29300a6c88ccd98256949ca7a6f6
WORKDIR /goroot/src
ENV GOHOSTARCH="$TARGETARCH"
RUN ./make.bash
ENV PATH="/goroot/bin:$PATH"
RUN go version

# https://getcomposer.org/doc/03-cli.md#composer-allow-superuser
ENV COMPOSER_ALLOW_SUPERUSER=1
COPY --from=composer/composer:2-bin /composer /usr/bin/composer

WORKDIR /go/src/app
COPY go.mod go.sum ./
RUN go mod graph | awk '{if ($1 !~ "@") print $2}' | xargs go get

WORKDIR /go/src/app/caddy
COPY caddy/go.mod caddy/go.sum ./
RUN go mod graph | awk '{if ($1 !~ "@") print $2}' | xargs go get

WORKDIR /go/src/app
COPY --link *.* ./
COPY --link caddy caddy
COPY --link internal internal

RUN --mount=type=secret,id=github-token GITHUB_TOKEN=$(cat /run/secrets/github-token) ./build-static.sh && \
	rm -Rf dist/static-php-cli/source/*
