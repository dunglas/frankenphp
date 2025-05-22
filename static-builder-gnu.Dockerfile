# syntax=docker/dockerfile:1
#checkov:skip=CKV_DOCKER_2
#checkov:skip=CKV_DOCKER_3
FROM centos:7

ARG FRANKENPHP_VERSION=''
ENV FRANKENPHP_VERSION=${FRANKENPHP_VERSION}

ARG BUILD_PACKAGES=''

ARG PHP_VERSION=''
ENV PHP_VERSION=${PHP_VERSION}

# args passed to static-php-cli
ARG PHP_EXTENSIONS=''
ARG PHP_EXTENSION_LIBS=''

# args passed to xcaddy
ARG XCADDY_ARGS=''
ARG CLEAN=''
ARG EMBED=''
ARG DEBUG_SYMBOLS=''
ARG MIMALLOC=''
ARG NO_COMPRESS=''

# Go
ARG GO_VERSION
ENV GOTOOLCHAIN=local

# labels, same as static-builder.Dockerfile
LABEL org.opencontainers.image.title=FrankenPHP
LABEL org.opencontainers.image.description="The modern PHP app server"
LABEL org.opencontainers.image.url=https://frankenphp.dev
LABEL org.opencontainers.image.source=https://github.com/dunglas/frankenphp
LABEL org.opencontainers.image.licenses=MIT
LABEL org.opencontainers.image.vendor="Kévin Dunglas"

# yum update
RUN sed -i 's/mirror.centos.org/vault.centos.org/g' /etc/yum.repos.d/*.repo && \
	sed -i 's/^#.*baseurl=http/baseurl=http/g' /etc/yum.repos.d/*.repo && \
	sed -i 's/^mirrorlist=http/#mirrorlist=http/g' /etc/yum.repos.d/*.repo && \
	yum clean all && \
	yum makecache && \
	yum update -y && \
	yum install -y centos-release-scl

# different arch for different scl repo
RUN if [ "$(uname -m)" = "aarch64" ]; then \
		sed -i 's|mirror.centos.org/centos|vault.centos.org/altarch|g' /etc/yum.repos.d/CentOS-SCLo-scl-rh.repo ; \
		sed -i 's|mirror.centos.org/centos|vault.centos.org/altarch|g' /etc/yum.repos.d/CentOS-SCLo-scl.repo ; \
		sed -i 's/^#.*baseurl=http/baseurl=http/g' /etc/yum.repos.d/*.repo ; \
		sed -i 's/^mirrorlist=http/#mirrorlist=http/g' /etc/yum.repos.d/*.repo ; \
	else \
		sed -i 's/mirror.centos.org/vault.centos.org/g' /etc/yum.repos.d/*.repo ; \
		sed -i 's/^#.*baseurl=http/baseurl=http/g' /etc/yum.repos.d/*.repo ; \
		sed -i 's/^mirrorlist=http/#mirrorlist=http/g' /etc/yum.repos.d/*.repo ; \
	fi; \
	yum update -y && \
	yum install -y devtoolset-10-gcc-* && \
	echo "source scl_source enable devtoolset-10" >> /etc/bashrc && \
	source /etc/bashrc

# install newer cmake to build some newer libs
RUN curl -o cmake.tar.gz -fsSL https://github.com/Kitware/CMake/releases/download/v3.31.4/cmake-3.31.4-linux-$(uname -m).tar.gz && \
	mkdir /cmake && \
	tar -xzf cmake.tar.gz -C /cmake --strip-components 1 && \
	rm cmake.tar.gz

# install build essentials
RUN yum install -y \
		perl \
		make \
		bison \
		flex \
		git \
		autoconf \
		automake \
		tar \
		unzip \
		gzip \
		gcc \
		bzip2 \
		patch \
		xz \
		libtool \
		perl-IPC-Cmd ; \
	curl -o make.tar.gz -fsSL https://ftp.gnu.org/gnu/make/make-4.4.tar.gz && \
	tar -zxvf make.tar.gz && \
	cd make-* && \
	./configure && \
	make && \
	make install && \
	ln -sf /usr/local/bin/make /usr/bin/make && \
	cd .. && \
	rm -Rf make* && \
	if [ "$(uname -m)" = "aarch64" ]; then \
		GO_ARCH="arm64" ; \
	else \
		GO_ARCH="amd64" ; \
	fi; \
	curl -o /usr/local/bin/jq -fsSL https://github.com/jqlang/jq/releases/download/jq-1.7.1/jq-linux-${GO_ARCH} && \
	chmod +x /usr/local/bin/jq && \
	curl -o go.tar.gz -fsSL https://go.dev/dl/$(curl -fsS https://go.dev/dl/?mode=json | jq -r "first(first(.[] | select(.stable and (.version | startswith(\"go${GO_VERSION}\")))).files[] | select(.os == \"linux\" and (.kind == \"archive\") and (.arch == \"${GO_ARCH}\"))).filename") && \
	rm -rf /usr/local/go && \
	tar -C /usr/local -xzf go.tar.gz && \
	rm go.tar.gz && \
	/usr/local/go/bin/go install github.com/caddyserver/xcaddy/cmd/xcaddy@latest

ENV PATH="/cmake/bin:/usr/local/go/bin:$PATH"

# Apply GNU mode
ENV CC='/opt/rh/devtoolset-10/root/usr/bin/gcc'
ENV CXX='/opt/rh/devtoolset-10/root/usr/bin/g++'
ENV AR='/opt/rh/devtoolset-10/root/usr/bin/ar'
ENV LD='/opt/rh/devtoolset-10/root/usr/bin/ld'
ENV SPC_DEFAULT_C_FLAGS='-fPIE -fPIC -O3'
ENV SPC_LIBC='glibc'
ENV SPC_CMD_VAR_PHP_MAKE_EXTRA_LDFLAGS_PROGRAM='-Wl,-O3 -pie'
ENV SPC_CMD_VAR_PHP_MAKE_EXTRA_LIBS='-ldl -lpthread -lm -lresolv -lutil -lrt'
ENV SPC_OPT_BUILD_ARGS='--with-config-file-path=/etc/frankenphp --with-config-file-scan-dir=/etc/frankenphp/php.d'
ENV SPC_REL_TYPE='binary'
ENV EXTENSION_DIR='/usr/lib/frankenphp/modules'

# not sure if this is needed
ENV COMPOSER_ALLOW_SUPERUSER=1

# install tools to build packages, if requested - needs gcc 10
RUN if [ -n "${BUILD_PACKAGES}" ]; then \
	yum install -y \
		bzip2 \
		libffi-devel \
		libyaml \
		libyaml-devel \
		make \
		openssl-devel \
		rpm-build \
		sudo \
		zlib-devel && \
	  curl -o ruby.tar.gz -fsSL https://cache.ruby-lang.org/pub/ruby/3.4/ruby-3.4.4.tar.gz && \
	  tar -xzf ruby.tar.gz && \
	  cd ruby-* && \
	  ./configure --without-baseruby && \
	  make && \
	  make install && \
	  cd .. && \
	  rm -rf ruby* && \
	  gem install fpm; \
fi

WORKDIR /go/src/app
COPY go.mod go.sum ./
RUN go mod download

WORKDIR /go/src/app/caddy
COPY caddy/go.mod caddy/go.sum ./
RUN go mod download

WORKDIR /go/src/app
COPY --link *.* ./
COPY --link caddy caddy
COPY --link internal internal
COPY --link package package

RUN --mount=type=secret,id=github-token \
	set -eux; \
	./build-static.sh && \
	if [ -n "${BUILD_PACKAGES}" ]; then \
		./build-packages.sh; \
	fi; \
	rm -Rf dist/static-php-cli/source/*
