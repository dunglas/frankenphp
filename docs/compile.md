# Compile From Sources

## Install PHP

To use FrankenPHP, you currently need to compile a fork of PHP.
Patches have been contributed upstream, and some have already
been merged. It will be possible to use the vanilla PHP interpreter starting with version 8.3.

First, get our PHP fork and prepare it:

```
git clone https://github.com/dunglas/php-src.git
cd php-src
git checkout frankenphp-8.2
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
`libiconv` and `bison`:

```
brew install libiconv bison
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
git clone git@github.com:dunglas/frankenphp.git
cd dunglas/frankenphp
git submodule update --init --recursive
cd caddy/frankenphp
go build
```
