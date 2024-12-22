#!/bin/bash

# install brendangregg's FlameGraph
if [ ! -d "/usr/local/src/flamegraph" ]; then
	mkdir /usr/local/src/flamegraph && \
		cd /usr/local/src/flamegraph && \
		git clone https://github.com/brendangregg/FlameGraph.git
fi

# let the test warm up
sleep 10

# run a 30 second profile on the Caddy admin port
cd /usr/local/src/flamegraph/FlameGraph && \
	go tool pprof -raw -output=cpu.txt 'http://localhost:2019/debug/pprof/profile?seconds=30' && \
	./stackcollapse-go.pl cpu.txt | ./flamegraph.pl > /go/src/app/testdata/performance/flamegraph.svg