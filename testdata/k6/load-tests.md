## Running Load tests

To run load tests with k6 you need to have docker installed

Go the root of this repository and run:

```sh
bash testdata/k6/load-test.sh
```

This will build the `frankenphp-dev` docker image and run it under the name 'load-test-container'
in the background. Additionally, it will download grafana/k6 and you'll be able to choose
the load test you want to run.