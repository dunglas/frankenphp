# FrankenPHP: Modern App Server for PHP

## Getting Started

```
docker run -v $PWD:/app/public \
    -p 80:80 -p 443:443 \
    dunglas/frankenphp
```

Your app is served at https://localhost!

## Docs

* [worker mode](docs/worker.md)
* [Early Hints support (103 HTTP status code)](docs/early-hints.md)
* [compile from sources](docs/compile.md)
