# Using FrankenPHP Workers

Boot your application once and keep it in memory.
FrankenPHP will handle incoming requests in a few milliseconds.

## Custom Apps

```php
<?php
// public/index.php

// Boot your app
require __DIR__.'/vendor/autoload.php';

$myApp = new \App\Kernel();
$myApp->boot();

do {
    $running = frankenphp_handle_request(function () use ($myApp) {
        // Called when a request is received,
        // superglobals, php://input and the like are reset
        echo $myApp->handle($_GET, $_POST, $_COOKIE, $_FILES, $_SERVER);
    });

    // Do something after sending the HTTP response
    $myApp->terminate();
} while ($running);

// Cleanup
$myApp->shutdown();
```

Then, start your app and use the `FRANKENPHP_CONFIG` environment variable to configure your worker: 

```sh
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 \
    dunglas/frankenphp
```

By default, one worker per CPU is started.
You can also configure the number of workers to start:

```sh
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php 42" \
    -v $PWD:/app \
    -p 80:80 -p 443:443 \
    dunglas/frankenphp
```
## Symfony Runtime

The worker mode of FrankenPHP is supported by the [Symfony Runtime Component](https://symfony.com/doc/current/components/runtime.html).
To start any Symfony application in a worker, install the FrankenPHP package of [PHP Runtime](https://github.com/php-runtime/runtime):

```sh
composer require runtime/frankenphp-symfony
```

Start your app server by defining the `APP_RUNTIME` environment variable to use the FrankenPHP Symfony Runtime

```sh
docker run \
    -e FRANKENPHP_CONFIG="worker ./public/index.php" \
    -e APP_RUNTIME=Runtime\\FrankenPhpSymfony\\Runtime \
    -v $PWD:/app \
    -p 80:80 -p 443:443 \
    dunglas/frankenphp
```

## Laravel Octane

Coming soon!
