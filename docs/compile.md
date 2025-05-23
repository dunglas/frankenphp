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
The following `./configure` flags are mandatory, but you can add others, for example, to compile extensions or additional features.

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
brew install libiconv bison brotli re2c pkg-config
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

## Install Optional Dependencies

Some FrankenPHP features depend on optional system dependencies that must be installed.
Alternatively, these features can be disabled by passing build tags to the Go compiler.

To install them, run ./install_dependencies.sh in the FrankenPHP source directory.

| Feature                        | Dependency                                                            | Build tag to disable it |
| ------------------------------ | --------------------------------------------------------------------- | ----------------------- |
| Brotli compression             | [Brotli](https://github.com/google/brotli)                            | nobrotli                |
| Restart workers on file change | [Watcher C](https://github.com/e-dant/watcher/tree/release/watcher-c) | nowatcher               |

## Compile the Go App

You can now build the binary:

```console
curl -L https://github.com/dunglas/frankenphp/archive/refs/heads/main.tar.gz | tar xz
cd frankenphp-main/caddy/frankenphp
CGO_CFLAGS="$(php-config --includes) -I$PWD/../../dist/dependencies/include" \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs) -L$PWD/../../dist/dependencies/lib" \
go build -tags=nobadger,nomysql,nopgx
```

Please keep in mind that this will produce a `frankenphp` binary without mercure and vulcain. For production use, prefer to use xcaddy.

### Using xcaddy

Alternatively, use [xcaddy](https://github.com/caddyserver/xcaddy) to compile FrankenPHP with [custom Caddy modules](https://caddyserver.com/docs/modules/):

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS="$(php-config --includes) -I$PWD/../../dist/dependencies/include" \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs) -L$PWD/../../dist/dependencies/lib" \
xcaddy build \
    --output frankenphp \
    --with github.com/dunglas/frankenphp/caddy \
    --with github.com/dunglas/mercure/caddy \
    --with github.com/dunglas/vulcain/caddy \
    --with github.com/dunglas/caddy-cbrotli
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
> (change the stack size value according to your app needs).
> Check the build-static.sh file for more information.
