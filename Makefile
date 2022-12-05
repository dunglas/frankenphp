.PHONY: build-dev start-serve-dev test

build-dev: 
	docker build -t frankenphp-dev -f Dockerfile.dev .

start-serve-dev: 
	docker run --cap-add=SYS_PTRACE --security-opt seccomp=unconfined -p 8080:8080 -p 443:443 -v ${PWD}:/go/src/app -it frankenphp-dev

test: 
	cd internal/testserver/
	go build
	cd ../../
	cd testdata/
	../internal/testserver/testserver
	curl -v http://127.0.0.1:8080/phpinfo.php

