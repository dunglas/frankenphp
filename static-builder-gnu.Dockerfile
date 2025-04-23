# syntax=docker/dockerfile:1
#checkov:skip=CKV_DOCKER_2
#checkov:skip=CKV_DOCKER_3
FROM centos:7

ARG FRANKENPHP_VERSION=''
ENV FRANKENPHP_VERSION=${FRANKENPHP_VERSION}

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

# go version
ENV GO_VERSION=1.24.1

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
    fi ; \
    yum update -y && \
    yum install -y devtoolset-10-gcc-* && \
    echo "source scl_source enable devtoolset-10" >> /etc/bashrc && \
    source /etc/bashrc

# install newer cmake to build some newer libs
RUN curl -o cmake.tgz -fsSL https://github.com/Kitware/CMake/releases/download/v3.31.4/cmake-3.31.4-linux-$(uname -m).tar.gz && \
    mkdir /cmake && \
    tar -xzf cmake.tgz -C /cmake --strip-components 1 && \
    rm cmake.tgz

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
    curl -o make.tgz -fsSL https://ftp.gnu.org/gnu/make/make-4.4.tar.gz && \
    tar -zxvf make.tgz && \
    rm make.tgz && \
    cd make-4.4 && \
    ./configure && \
    make && \
    make install && \
    ln -sf /usr/local/bin/make /usr/bin/make ; \
    if [ "$(uname -m)" = "aarch64" ]; then \
        GO_ARCH="arm64" ; \
    else \
        GO_ARCH="amd64" ; \
    fi ; \
    curl -o jq -fsSL https://github.com/jqlang/jq/releases/download/jq-1.7.1/jq-linux-${GO_ARCH} && \
    chmod +x jq && \
    mv jq /usr/local/bin/jq && \
    curl -o go.tgz -fsSL https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz && \
    rm -rf /usr/local/go && \
    tar -C /usr/local -xzf go.tgz && \
    rm go.tgz && \
    /usr/local/go/bin/go install github.com/caddyserver/xcaddy/cmd/xcaddy@latest

ENV PATH="/cmake/bin:/usr/local/go/bin:$PATH"

# Apply gnu mode
ENV CC='/opt/rh/devtoolset-10/root/usr/bin/gcc'
ENV CXX='/opt/rh/devtoolset-10/root/usr/bin/g++'
ENV AR='/opt/rh/devtoolset-10/root/usr/bin/ar'
ENV LD='/opt/rh/devtoolset-10/root/usr/bin/ld'
ENV SPC_DEFAULT_C_FLAGS='-fPIE -fPIC -O3'
ENV SPC_LIBC='glibc'
ENV SPC_CMD_VAR_PHP_MAKE_EXTRA_LDFLAGS_PROGRAM='-Wl,-O3 -pie'
ENV SPC_CMD_VAR_PHP_MAKE_EXTRA_LIBS='-ldl -lpthread -lm -lresolv -lutil -lrt'
ENV SPC_OPT_DOWNLOAD_ARGS='--ignore-cache-sources=php-src'
ENV SPC_OPT_BUILD_ARGS=''
ENV SPC_REL_TYPE='binary'

# not sure if this is needed
ENV COMPOSER_ALLOW_SUPERUSER=1

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

RUN --mount=type=secret,id=github-token ./build-static.sh && \
	rm -Rf dist/static-php-cli/source/*
