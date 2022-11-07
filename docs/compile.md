# Compile From Sources

## Install PHP

FrankenPHP is compatible with the PHP 8.2 and superior.

First, get the sources of PHP:

```
curl -L https://github.com/php/php-src/archive/refs/heads/PHP-8.2.tar.gz | tar xz
cd php-src-PHP-8.2
./buildconf
```

Then, configure PHP for your platform:

### Linux

```
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals
```

### Mac

Use the [Homebrew](https://brew.sh/) package manager to install
`libiconv`, `bison` and `re2c`:

```
brew install libiconv bison re2c
echo 'export PATH="/opt/homebrew/opt/bison/bin:$PATH"' >> ~/.zshrc
```

Then run the configure script:

```
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

## Compile PHP

Finally, compile PHP:

```
make -j$(nproc)
make install
```

#### Compile the Go App

You can now use the Go lib and compile our Caddy build:

```
git clone --recursive git@github.com:dunglas/frankenphp.git
cd frankenphp/caddy/frankenphp
go build
```
