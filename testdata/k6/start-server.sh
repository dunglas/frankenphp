#!/bin/bash

# run the load test server with the k6.Caddyfile

cd /go/src/app/caddy/frankenphp \
&& go build --buildvcs=false \
&& cd ../../testdata/k6 \
&& /go/src/app/caddy/frankenphp/frankenphp run -c k6.Caddyfile