# Contributing

## Compiling PHP
### With Docker (Linux)

Build the dev Docker image:

    docker build -t frankenphp-dev -f Dockerfile.dev .
    docker run --cap-add=SYS_PTRACE --security-opt seccomp=unconfined -p 8080:8080 -p 443:443 -v $PWD:/go/src/app -it frankenphp-dev

The image contains the usual development tools (Go, GDB, Valgrind, Neovim...).

### Without Docker (Linux and macOS)

[Follow the instructions to compile from sources](docs/compile.md) and pass the `--debug` configuration flag.

## Running the test suite

    go test -race -v ./...

## Caddy module

Build Caddy with the FrankenPHP Caddy module:

    cd caddy/frankenphp/
    go build
    cd ../../

Run the Caddy with the FrankenPHP Caddy module:

    cd testdata/
    ../caddy/frankenphp/frankenphp run

The server is listening on `127.0.0.1:8080`:

    curl -vk https://localhosy/phpinfo.php

## Minimal test server

Build the minimal test server:

    cd internal/testserver/
    go build
    cd ../../

Run the test server:

    cd testdata/
    ../internal/testserver/testserver

The server is listening on `127.0.0.1:8080`:

    curl -v http://127.0.0.1:8080/phpinfo.php

# Building Docker Images Locally

Print bake plan:

```
docker buildx bake -f docker-bake.hcl --print
```

Build FrankenPHP images for amd64 locally:

```
docker buildx bake -f docker-bake.hcl --pull --load --set "*.platform=linux/amd64"
```

Build FrankenPHP images for arm64 locally:

```
docker buildx bake -f docker-bake.hcl --pull --load --set "*.platform=linux/arm64"
```

Build FrankenPHP images from scratch for arm64 & amd64 and push to Docker Hub:

```
docker buildx bake -f docker-bake.hcl --pull --no-cache --push
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

## Docker-Related Resources

* [Bake file definition](https://docs.docker.com/build/customize/bake/file-definition/)
* [docker buildx build](https://docs.docker.com/engine/reference/commandline/buildx_build/)
