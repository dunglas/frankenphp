# Compile From Sources

## Install PHP

FrankenPHP is compatible with the PHP 8.2 and superior.

First, [get the sources of PHP](https://www.php.net/downloads.php) and extract them:

```
tar xf php-*
cd php-*/
```

Then, configure PHP for your platform:

### Linux

```
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --enable-zend-max-execution-timers
```

### Mac

Use the [Homebrew](https://brew.sh/) package manager to install
`libiconv`, `bison`, `re2c` and `pkg-config`:

```
brew install libiconv bison re2c pkg-config
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
sudo make install
```

#### Compile the Go App

You can now use the Go lib and compile our Caddy build:

```
curl -L https://github.com/dunglas/frankenphp/archive/refs/heads/main.tar.gz | tar x
cd frankenphp-main/caddy/frankenphp
CGO_CFLAGS=$(php-config --includes) go build
```
