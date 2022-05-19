# Caddy PHP


## Install

### Docker

The easiest way to get started is to use our Docker image:

```
docker build -t frankenphp .
```

### Compile From Sources

#### Install PHP

Most distributions don't provide packages containing ZTS builds of PHP.
Because the Go HTTP server uses goroutines, a ZTS build is needed.

Start by [downloading the latest version of PHP](https://www.php.net/downloads.php),
then follow the instructions according to your operating system.

##### Linux

```
./configure \
    --enable-embed=static \
    --enable-zts
make -j6
make install
```

##### Mac

The instructions to build on Mac and Linux are similar.
However, on Mac, you have to use the [Homebrew](https://brew.sh/) package manager to install `libiconv` and `bison`.
You also need to slightly tweak the configuration.

```
brew install libiconv bison
echo 'export PATH="/opt/homebrew/opt/bison/bin:$PATH"' >> ~/.zshrc
./configure \
    --enable-embed=static \
    --enable-zts \
    --with-iconv=/opt/homebrew/opt/libiconv/ \
    --without-pcre-jit
make -j6
make install
```

#### Compile the Go App

```
go get -d -v ./...
go build -v
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
