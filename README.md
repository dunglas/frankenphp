# Caddy PHP


## Install

### Docker

The easiest way to get started is to use our Docker image:

```
docker build -t frankenphp .
```

### Compile fron Sources

#### Install PHP

Most distributions doesn't provide packages containing ZTS builds of PHP.
Because the Go HTTP server uses goroutines, a ZTS build is needed.

Start by [downloading the latest version of PHP](https://www.php.net/downloads.php),
then follow the instructions according to your operating system.

##### Linux

```
./configure --enable-embed --enable-zts
make
make install
```

##### Mac

```
brew install libiconv
./configure \
    --enable-zts \
    --enable-embed=dylib \
    --with-iconv=/opt/homebrew/opt/libiconv/ \
    --without-pcre-jit
make
make install
```

Then, you also need to build a Mac-compatible PHP shared library.
As the standard PHP distribution doesn't provide one, you need to do a few extra steps:

Start by adding those lines at the end of the `Makefile`:

```
libs/libphp.dylib: $(PHP_GLOBAL_OBJS) $(PHP_SAPI_OBJS)
	$(LIBTOOL) --mode=link $(CC) -dynamiclib $(LIBPHP_CFLAGS) $(CFLAGS_CLEAN) $(EXTRA_CFLAGS) -rpath $(phptempdir) $(EXTRA_LDFLAGS) $(LDFLAGS) $(PHP_RPATHS) $(PHP_GLOBAL_OBJS) $(PHP_SAPI_OBJS) $(EXTRA_LIBS) $(ZEND_EXTRA_LIBS) -o $@
	-@$(LIBTOOL) --silent --mode=install cp $@ $(phptempdir)/$@ >/dev/null 2>&1
```

Then run:

```
make libs/libphp.dylib
sudo cp libs/libphp.dylib /usr/local/lib/
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
