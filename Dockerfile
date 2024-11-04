# syntax=docker/dockerfile:1
#checkov:skip=CKV_DOCKER_2
#checkov:skip=CKV_DOCKER_3
#checkov:skip=CKV_DOCKER_7
FROM php-base AS common

WORKDIR /app

RUN apt-get update && \
	apt-get -y --no-install-recommends install \
		mailcap \
		libcap2-bin \
	&& \
	apt-get clean && \
	rm -rf /var/lib/apt/lists/*

RUN set -eux; \
	mkdir -p \
		/app/public \
		/config/caddy \
		/data/caddy \
		/etc/caddy; \
	sed -i 's/php/frankenphp run/g' /usr/local/bin/docker-php-entrypoint; \
	echo '<?php phpinfo();' > /app/public/index.php

COPY --link caddy/frankenphp/Caddyfile /etc/caddy/Caddyfile
RUN curl -sSLf \
		-o /usr/local/bin/install-php-extensions \
		https://github.com/mlocati/docker-php-extension-installer/releases/latest/download/install-php-extensions && \
	chmod +x /usr/local/bin/install-php-extensions

CMD ["--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile"]
HEALTHCHECK CMD curl -f http://localhost:2019/metrics || exit 1

# See https://caddyserver.com/docs/conventions#file-locations for details
ENV XDG_CONFIG_HOME=/config
ENV XDG_DATA_HOME=/data

EXPOSE 80
EXPOSE 443
EXPOSE 443/udp
EXPOSE 2019

LABEL org.opencontainers.image.title=FrankenPHP
LABEL org.opencontainers.image.description="The modern PHP app server"
LABEL org.opencontainers.image.url=https://frankenphp.dev
LABEL org.opencontainers.image.source=https://github.com/dunglas/frankenphp
LABEL org.opencontainers.image.licenses=MIT
LABEL org.opencontainers.image.vendor="Kévin Dunglas"


FROM common AS builder

ARG FRANKENPHP_VERSION='dev'
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

COPY --from=golang-base /usr/local/go /usr/local/go

ENV PATH=/usr/local/go/bin:$PATH

# This is required to link the FrankenPHP binary to the PHP binary
RUN apt-get update && \
	apt-get -y --no-install-recommends install \
    cmake \
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
	&& \
	apt-get clean

WORKDIR /go/src/app

COPY --link go.mod go.sum ./
RUN go mod graph | awk '{if ($1 !~ "@") print $2}' | xargs go get

WORKDIR /go/src/app/caddy
COPY --link caddy/go.mod caddy/go.sum ./
RUN go mod graph | awk '{if ($1 !~ "@") print $2}' | xargs go get

WORKDIR /go/src/app
COPY --link *.* ./
COPY --link caddy caddy
COPY --link internal internal
COPY --link testdata testdata

# Install e-dant/watcher (necessary for file watching)
WORKDIR /usr/local/src/watcher
RUN curl -s https://api.github.com/repos/e-dant/watcher/releases/latest | \
		grep tarball_url | \
		awk '{ print $2 }' | \
		sed 's/,$//' | \
		sed 's/"//g' | \
		xargs curl -L | \
    tar xz --strip-components 1 && \
    cmake -S . -B build -DCMAKE_BUILD_TYPE=Release && \
	cmake --build build && \
	cmake --install build && \
    ldconfig

# See https://github.com/docker-library/php/blob/master/8.3/bookworm/zts/Dockerfile#L57-L59 for PHP values
ENV CGO_CFLAGS="-DFRANKENPHP_VERSION=$FRANKENPHP_VERSION $PHP_CFLAGS"
ENV CGO_CPPFLAGS=$PHP_CPPFLAGS
ENV CGO_LDFLAGS="-L/usr/local/lib -lssl -lcrypto -lreadline -largon2 -lcurl -lonig -lz $PHP_LDFLAGS"

RUN echo $CGO_LDFLAGS

WORKDIR /go/src/app/caddy/frankenphp
RUN GOBIN=/usr/local/bin go install -tags 'nobadger,nomysql,nopgx' -ldflags "-w -s -X 'github.com/caddyserver/caddy/v2.CustomVersion=FrankenPHP $FRANKENPHP_VERSION PHP $PHP_VERSION Caddy'" && \
	setcap cap_net_bind_service=+ep /usr/local/bin/frankenphp && \
	cp Caddyfile /etc/caddy/Caddyfile && \
	frankenphp version

WORKDIR /go/src/app


FROM common AS runner

ENV GODEBUG=cgocheck=0

# copy watcher shared library
COPY --from=builder /usr/local/lib/libwatcher* /usr/local/lib/
# fix for the file watcher on arm
RUN apt-get install -y --no-install-recommends libstdc++6 && \
	apt-get clean && \
	ldconfig

COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp
RUN setcap cap_net_bind_service=+ep /usr/local/bin/frankenphp && \
	frankenphp version
