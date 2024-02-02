# FrankenPHP: Modern App Server for PHP

<h1 align="center"><a href="https://frankenphp.dev"><img src="frankenphp.png" alt="FrankenPHP" width="600"></a></h1>

FrankenPHP is a modern application server for PHP built on top of the [Caddy](https://caddyserver.com/) web server.

FrankenPHP gives superpowers to your PHP apps thanks to its stunning features: [*Early Hints*](https://frankenphp.dev/docs/early-hints/), [worker mode](https://frankenphp.dev/docs/worker/), [real-time capabilities](https://frankenphp.dev/docs/mercure/), automatic HTTPS, HTTP/2, and HTTP/3 support...

FrankenPHP works with any PHP app and makes your Symfony and Laravel projects faster than ever thanks to the provided integration with the worker mode.

FrankenPHP can also be used as a standalone Go library to embed PHP in any app using `net/http`.

[**Learn more** on *frankenphp.dev*](https://frankenphp.dev) and in this slide deck:

<a href="https://dunglas.dev/2022/10/frankenphp-the-modern-php-app-server-written-in-go/"><img src="https://dunglas.dev/wp-content/uploads/2022/10/frankenphp.png" alt="Slides" width="600"></a>

## Getting Started

### Docker

```console
docker run -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

Go to `https://localhost`, and enjoy!

> [!TIP]
>
> Do not attempt to use `https://127.0.0.1`. Use `localhost` and accept the self-signed certificate.
> Use the [`SERVER_NAME` environment variable](docs/config.md#environment-variables) to change the domain to use.

### Standalone Binary

If you prefer not to use Docker, we provide standalone FrankenPHP binaries for Linux and macOS
containing [PHP 8.3](https://www.php.net/releases/8.3/en.php) and most popular PHP extensions: [Download FrankenPHP](https://github.com/dunglas/frankenphp/releases)

To serve the content of the current directory, run:

```console
./frankenphp php-server
```

You can also run command-line scripts with:

```console
./frankenphp php-cli /path/to/your/script.php
```

## Docs

* [The worker mode](https://frankenphp.dev/docs/worker/)
* [Early Hints support (103 HTTP status code)](https://frankenphp.dev/docs/early-hints/)
* [Real-time](https://frankenphp.dev/docs/mercure/)
* [Configuration](https://frankenphp.dev/docs/config/)
* [Docker images](https://frankenphp.dev/docs/docker/)
* [Deploy in production](docs/production.md)
* [Create **standalone**, self-executable PHP apps](https://frankenphp.dev/docs/embed/)
* [Create static binaries](https://frankenphp.dev/docs/static/)
* [Compile from sources](https://frankenphp.dev/docs/compile/)
* [Laravel integration](https://frankenphp.dev/docs/laravel/)
* [Known issues](https://frankenphp.dev/docs/known-issues/)
* [Demo app (Symfony) and benchmarks](https://github.com/dunglas/frankenphp-demo)
* [Go library documentation](https://pkg.go.dev/github.com/dunglas/frankenphp)
* [Contributing and debugging](https://frankenphp.dev/docs/contributing/)

## Examples and Skeletons

* [Symfony](https://github.com/dunglas/symfony-docker)
* [API Platform](https://api-platform.com/docs/distribution/)
* [Laravel](https://frankenphp.dev/docs/laravel/)
* [Sulu](https://sulu.io/blog/running-sulu-with-frankenphp)
* [WordPress](https://github.com/dunglas/frankenphp-wordpress)
* [Drupal](https://github.com/dunglas/frankenphp-drupal)
* [Joomla](https://github.com/alexandreelise/frankenphp-joomla)
* [TYPO3](https://github.com/ochorocho/franken-typo3)
