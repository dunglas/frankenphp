# Compile From Sources

This document explain how to create a FrankenPHP build that will load PHP as a dymanic library.
This is the recommended method.

Alternatively, [creating static builds](static.md) is also possible.

## Install PHP

FrankenPHP is compatible with the PHP 8.2 and superior.

First, [get the sources of PHP](https://www.php.net/downloads.php) and extract them:

```console
tar xf php-*
cd php-*/
```

Then, configure PHP for your platform:

### Linux

```console
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --enable-zend-max-execution-timers
```

Finally, compile and install PHP:

```console
make -j$(nproc)
sudo make install
```

### Mac

Use the [Homebrew](https://brew.sh/) package manager to install
`libiconv`, `bison`, `re2c` and `pkg-config`:

```console
brew install libiconv bison re2c pkg-config
echo 'export PATH="/opt/homebrew/opt/bison/bin:$PATH"' >> ~/.zshrc
```

Then run the configure script:

```console
./configure \
    --enable-embed=static \
    --enable-zts \
    --disable-zend-signals \
    --disable-opcache-jit \
    --enable-static \
    --enable-shared=no \
    --with-iconv=/opt/homebrew/opt/libiconv/
```

These flags are required, but you can add other flags (e.g. extra extensions)
if needed.

Finally, compile and install PHP:

```console
make -j$(sysctl -n hw.logicalcpu)
sudo make install
```

## Compile the Go App

You can now use the Go library and compile our Caddy build:

```console
curl -L https://github.com/dunglas/frankenphp/archive/refs/heads/main.tar.gz | tar x
cd frankenphp-main/caddy/frankenphp
CGO_CFLAGS=$(php-config --includes) CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" go build
```

### Using xcaddy

Alternatively, use [xcaddy](https://github.com/caddyserver/xcaddy) to compile FrankenPHP with [custom Caddy modules](https://caddyserver.com/docs/modules/):

```console
CGO_ENABLED=1 xcaddy build \
    --output frankenphp \
    --with github.com/dunglas/frankenphp/caddy \
    --with github.com/dunglas/mercure/caddy \
    --with github.com/dunglas/vulcain/caddy
    # Add extra Caddy modules here
```
