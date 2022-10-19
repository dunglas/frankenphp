# Contributing...

## Running the test suite

    go test -race -v ./...

## Debugging
### With Docker (Linux)

Build the dev Docker image:

    docker build -t frankenphp-dev Dockerfile.dev 
    docker run -p 8080:8080 -p 443:443 -v $PWD:/go/src/app -it frankenphp-dev bash

The image contains the usual development tools (Go, GDB, Valgrind, Neovim...).
#### Caddy module

Build Caddy with the FrankenPHP Caddy module:

    cd /go/src/app/caddy/frankenphp/
    go build

Run the Caddy with the FrankenPHP Caddy module:

    cd /go/src/app/testdata/
    ../caddy/frankenphp/frankenphp run

#### Minimal test server

Build the minimal test server:

    cd /go/src/app/internal/testserver/
    go build

Run the test server:

    cd /go/src/app/testdata/
    ../internal/testserver/testserver

The server is listening on `127.0.0.1:8080`:

    curl http://127.0.0.1:8080/phpinfo.php

### Without Docker (Linux and macOS)

Compile PHP:

    ./configure --enable-debug --enable-zts
    make -j6
    sudo make install

Build the minimal test server:

    cd internal/testserver/
    go build

Run the test app:

    cd ../../testdata/
    ../internal/testserver/testserver

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
