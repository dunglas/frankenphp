#!/bin/bash

# install the dev.Dockerfile, build the app and run k6 tests

docker build -t frankenphp-dev -f dev.Dockerfile .

export "CADDY_HOSTNAME=http://host.docker.internal"

select filename in ./testdata/k6/*.js; do
    read -p "How many worker threads? " workerThreads
    read -p "How many num threads? (must be > worker threads) " numThreads
    read -p "How many max threads? " maxThreads

    docker run --cap-add=SYS_PTRACE --security-opt seccomp=unconfined \
    -p 8125:80 \
    -v $PWD:/go/src/app \
    --name load-test-container \
    -e "MAX_THREADS=$maxThreads" \
    -e "WORKER_THREADS=$workerThreads" \
    -e "NUM_THREADS=$numThreads" \
    -itd \
    frankenphp-dev \
    sh /go/src/app/testdata/k6/start-server.sh

    docker exec -d load-test-container sh /go/src/app/testdata/k6/flamegraph.sh

    sleep 10

    docker run --entrypoint "" -it  -v .:/app -w /app \
    --add-host "host.docker.internal:host-gateway" \
    grafana/k6:latest \
    k6 run -e "CADDY_HOSTNAME=$CADDY_HOSTNAME:8125" "./$filename"

    docker exec load-test-container curl "http://localhost:2019/frankenphp/threads"

    docker stop load-test-container
    docker rm load-test-container
done

