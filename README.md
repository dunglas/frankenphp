# Caddy PHP


## Install

### Docker

The easiest way to get started is to use our Docker image:

```
docker build -t frankenphp .
```

### Compile From Sources

#### Install PHP

To use FrankenPHP, you currently need to compile a fork of PHP.
Patches have been contributed upstream, and some have already
been merged. It will be possible to use the vanilla version of PHP
starting with version 8.3.

First, get our PHP fork and prepare it:

```
git clone https://github.com/dunglas/php-src.git
cd php-src
git checkout frankenphp-8.2
./buildconf
```

Then, configure PHP for your platform:

**Linux**:

```
./configure \
    --enable-embed \
    --enable-zts \
    --disable-zend-signals
```

**Mac**:

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
    --with-iconv=/opt/homebrew/opt/libiconv/
```

These flags are required, but you can add other flags (extra extensions...)
if needed.

Finally, compile PHP:

```
make -j6
make install
```

#### Compile the Go App

You can now use the Go lib and compile our Caddy build:

```
cd caddy/frankenphp
go build
```

## Misc Dev Resources

* [PHP embedding in uWSGI](https://github.com/unbit/uwsgi/blob/master/plugins/php/php_plugin.c)
* [PHP embedding in NGINX Unit](https://github.com/nginx/unit/blob/master/src/nxt_php_sapi.c)
* [PHP embedding in Go (go-php)](https://github.com/deuill/go-php)
* [PHP embedding in Go (GoEmPHP)](https://github.com/mikespook/goemphp)
* [PHP embedding in C++](https://gist.github.com/paresy/3cbd4c6a469511ac7479aa0e7c42fea7)
* [Extending and Embedding PHP by Sara Golemon](https://books.google.fr/books?id=zMbGvK17_tYC&pg=PA254&lpg=PA254#v=onepage&q&f=false)
* [What the heck is TSRMLS_CC, anyway?](http://blog.golemon.com/2006/06/what-heck-is-tsrmlscc-anyway.html)
* [PHP embedding on Mac](https://gist.github.com/jonnywang/61427ffc0e8dde74fff40f479d147db4)
* [SDL bindings](https://pkg.go.dev/github.com/veandco/go-sdl2@v0.4.21/sdl#Main)
