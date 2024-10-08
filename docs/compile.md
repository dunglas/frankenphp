# Compile From Sources

This document explains how to create a FrankenPHP binary that will load PHP as a dynamic library.
This is the recommended method.

Alternatively, [static builds](static.md) can also be created.

## Install PHP

FrankenPHP is compatible with PHP 8.2 and superior.

First, [get the PHP sources](https://www.php.net/downloads.php) and extract them:

```console
tar xf php-*
cd php-*/
```

Then, run the `configure` script with the options needed for your platform.
Th following `./configure` flags are mandatory, but you can add others, for example to compile extensions or additional features.

### Linux

```console
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals \
    --enable-zend-max-execution-timers
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

## Compile PHP

Finally, compile and install PHP:

```console
make -j"$(getconf _NPROCESSORS_ONLN)"
sudo make install
```

## Compile the Go App

You can now build the final binary:

```console
curl -L https://github.com/dunglas/frankenphp/archive/refs/heads/main.tar.gz | tar xz
cd frankenphp-main/caddy/frankenphp
CGO_CFLAGS=$(php-config --includes) CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" go build
```

### Using xcaddy

Alternatively, use [xcaddy](https://github.com/caddyserver/xcaddy) to compile FrankenPHP with [custom Caddy modules](https://caddyserver.com/docs/modules/):

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags '-w -s'" \
xcaddy build \
    --output frankenphp \
    --with github.com/dunglas/frankenphp/caddy \
    --with github.com/dunglas/mercure/caddy \
    --with github.com/dunglas/vulcain/caddy
    # Add extra Caddy modules here
```

> [!TIP]
>
> If you're using musl libc (the default on Alpine Linux) and Symfony,
> you may need to increase the default stack size.
> Otherwise, you may get errors like `PHP Fatal error: Maximum call stack size of 83360 bytes reached during compilation. Try splitting expression`
>
> To do so, change the `XCADDY_GO_BUILD_FLAGS` environment variable to something like
> `XCADDY_GO_BUILD_FLAGS=$'-ldflags "-w -s -extldflags \'-Wl,-z,stack-size=0x80000\'"'`
> (change the value of the stack size according to your app needs).

## Build Tags

Additional features can be enabled if the required C libraries are installed by
passing additional build tags to the Go compiler:

| Tag     | Dependencies                                 | Description                    |
|---------|----------------------------------------------|--------------------------------|
| brotli  | [Brotli](https://github.com/google/brotli)   | Brotli compression             |
| watcher | [Watcher](https://github.com/e-dant/watcher) | Restart workers on file change |

When using `go build` directly, pass the additional `-tags` option followed by the comma-separated list of tags:

```console
go build -tags 'brotli watcher'
```

When using `xcaddy`, set the `-tags` option in the `XCADDY_GO_BUILD_FLAGS` environment variable:

```console
XCADDY_GO_BUILD_FLAGS="-tags 'brotli watcher'"
```
