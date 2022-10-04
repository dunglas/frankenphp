# Contributing

## Running the test suite

    go test -race -v ./...

## Testing in live
### With Docker (Linux)

Prepare a dev Docker image:

    docker build -t frankenphp .
    docker run -p 8080:8080 -p 443:443 -v $PWD:/go/src/app -it frankenphp bash

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
