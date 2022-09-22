FROM golang

ARG PHP_VERSION=8.1.7
ARG LIBICONV_VERSION=1.17
ENV PHPIZE_DEPS \
		autoconf \
		dpkg-dev \
		file \
		g++ \
		gcc \
		libc-dev \
		make \
		pkg-config \
		re2c

# Sury doesn't provide ZTS builds for now
#RUN apt-get update && \
#    apt-get -y --no-install-recommends install apt-transport-https lsb-release && \
#    wget -O /etc/apt/trusted.gpg.d/php.gpg https://packages.sury.org/php/apt.gpg && \
#    sh -c 'echo "deb https://packages.sury.org/php/ $(lsb_release -sc) main" > /etc/apt/sources.list.d/php.list' && \
#    apt-get update && \
#    apt-get -y --no-install-recommends install php8.1-dev && \
#    apt-get -y remove apt-transport-https lsb-release && \
#    apt-get clean all
#ENV CGO_CFLAGS="-I /usr/include/php/20200930 -I /usr/include/php/20200930/Zend -I /usr/include/php/20200930/TSRM -I /usr/include/php/20200930/main -I /usr/include/php/20200930/sapi/embed"

# TODO: check the downloaded package using the provided GPG signatures
RUN apt-get update && \
    apt-get -y --no-install-recommends install \
        $PHPIZE_DEPS \
        libargon2-dev \
		libcurl4-openssl-dev \
		libonig-dev \
		libreadline-dev \
		libsodium-dev \
		libsqlite3-dev \
		libssl-dev \
		libxml2-dev \
		zlib1g-dev \
        # Dev tools \
        gdb \
        neovim && \
        echo 'set auto-load safe-path /' > /root/.gdbinit && \
        echo '* soft core unlimited' >> /etc/security/limits.conf \
     && \
    apt-get clean && \
    curl -s -o php-${PHP_VERSION}.tar.gz https://www.php.net/distributions/php-${PHP_VERSION}.tar.gz && \
    tar -xf php-${PHP_VERSION}.tar.gz && \
    cd php-${PHP_VERSION}/ && \
    # --enable-embed is only necessary to generate libphp.so, we don't use this SAPI directly
    ./configure --enable-zts --enable-embed=static --enable-debug && \
    make && \
    make install && \
    rm -Rf php-${PHP_VERSION}/ php-${PHP_VERSION}.tar.gz && \
    ldconfig && \
    php --version

WORKDIR /go/src/app

COPY . .

RUN go get -d -v ./...
#RUN go build -v
#RUN cd cmd/frankenphp && go install -v ./...

#CMD ["frankenphp"]
