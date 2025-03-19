FROM centos:7

ARG TARGETARCH
ARG GOARCH

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

RUN BASE_ARCH=$(uname -m); \
    if [ "$BASE_ARCH" = "arm64" ]; then \
        TARGETARCH=aarch64; \
        GOARCH = arm64; \
    else \
        TARGETARCH=amd64; \
        GOARCH = amd64; \
    fi

RUN sed -i 's/mirror.centos.org/vault.centos.org/g' /etc/yum.repos.d/*.repo && \
    sed -i 's/^#.*baseurl=http/baseurl=http/g' /etc/yum.repos.d/*.repo && \
    sed -i 's/^mirrorlist=http/#mirrorlist=http/g' /etc/yum.repos.d/*.repo

RUN yum install -y centos-release-scl

RUN if [ "$TARGETARCH" = "aarch64" ]; then \
        sed -i 's|mirror.centos.org/centos|vault.centos.org/altarch|g' /etc/yum.repos.d/CentOS-SCLo-scl-rh.repo ; \
        sed -i 's|mirror.centos.org/centos|vault.centos.org/altarch|g' /etc/yum.repos.d/CentOS-SCLo-scl.repo ; \
    else \
        sed -i 's/mirror.centos.org/vault.centos.org/g' /etc/yum.repos.d/*.repo ; \
    fi
RUN sed -i 's/^#.*baseurl=http/baseurl=http/g' /etc/yum.repos.d/*.repo && \
    sed -i 's/^mirrorlist=http/#mirrorlist=http/g' /etc/yum.repos.d/*.repo

RUN yum clean all && \
    yum makecache && \
    yum update -y && \
    yum install -y devtoolset-10-gcc-* && \
    localedef -c -i en_US -f UTF-8 en_US.UTF-8

RUN echo "source scl_source enable devtoolset-10" >> /etc/bashrc
RUN source /etc/bashrc

# Install CMake
RUN curl -o cmake.tgz -fsSL https://github.com/Kitware/CMake/releases/download/v3.31.4/cmake-3.31.4-linux-amd64.tar.gz && \
    mkdir /cmake && \
    tar -xzf cmake.tgz -C /cmake --strip-components 1

RUN curl -o make.tgz -fsSL https://ftp.gnu.org/gnu/make/make-4.4.tar.gz && \
    tar -zxvf make.tgz && \
    cd make-4.4 && \
    ./configure && \
    make && \
    make install && \
    ln -sf /usr/local/bin/make /usr/bin/make

RUN curl -o automake.tgz -fsSL https://ftp.gnu.org/gnu/automake/automake-1.17.tar.xz && \
    tar -xvf automake.tgz && \
    cd automake-1.17 && \
    ./configure && \
    make && \
    make install && \
    ln -sf /usr/local/bin/automake /usr/bin/automake

# Install Go
RUN curl -o go.tgz -fsSL https://go.dev/dl/go1.24.1.linux-$GOARCH && \
    rm -rf /usr/local/go && tar -C /usr/local -xzf go.tgz \

ENV PATH="/cmake/bin:/usr/local/go/bin:$PATH"
ENV CC=/opt/rh/devtoolset-10/root/usr/bin/gcc
ENV CXX=/opt/rh/devtoolset-10/root/usr/bin/g++
ENV AR=/opt/rh/devtoolset-10/root/usr/bin/ar
ENV LD=/opt/rh/devtoolset-10/root/usr/bin/ld
ENV SPC_DEFAULT_C_FLAGS="-fPIE -fPIC"
ENV SPC_LIBC=glibc
ENV SPC_CMD_VAR_PHP_MAKE_EXTRA_LDFLAGS_PROGRAM="-Wl,-O1 -pie"
ENV SPC_CMD_VAR_PHP_MAKE_EXTRA_LIBS="-ldl -lpthread -lm -lresolv -lutil -lrt"

RUN --mount=type=secret,id=github-token GITHUB_TOKEN=$(cat /run/secrets/github-token) ./build-static-glibc.sh && \
	rm -Rf dist/static-php-cli/source/*

