# Performance

By default, FrankenPHP tries to offer a good compromise between performance and ease of use.
However, it is possible to slightly improve performance using the appropriate configuration.

## Number of Threads and of Workers

By default, FrankenPHP starts 2 times more threads and workers (in worker mode) than the available numbers of CPU.

The appropriate values depend heavily on how your application is written, what it does and your hardware.
We strongly recommend to change these values.

To find the right values, it's best to try out different values and run load tests simulating real traffic.
[k6](https://k6.io) and [Gatling](https://gatling.io) are good tools for this.

## Worker Mode

Enabling [the worker mode](worker.md) will dramatically improve the performance,
but your app must be adapted to be comptible with this mode:
you need to create a worker script and to be sure that the app is not leaking memory.

## Go Runtime Configuration

FrankenPHP is written in Go.

In general, the Go runtime doesn't require any special configuration, but in certain circumstances it can be helped to perform better.

You likely want to set the `GODEBUG` environement variable to `cgocheck=0` (the default in the FrankenPHP Docker images).

If you run FrankenPHP in containers (Docker, Kubernetes, LXC...) and limit the memory available for the containers,
set the `GOMEMLIMIT` environement variable to the available amount of memory.

For more details, [the Go documentation page dedicated to this subject](https://pkg.go.dev/runtime#hdr-Environment_Variables) is a must-read to get the most out of the runtime.

## `file_server`

By default, the `php_server` directive automatically sets up a file server to
serve static files (assets) stored in the root directory.

This feature is convenient, but comes with a cost.
To disable it, use the following config:

```caddyfile
php_server {
    file_server off
}
```

## Placeholders

You can use [placeholders](https://caddyserver.com/docs/conventions#placeholders) in the `root` and `env` directives.
However, this prevent caching these values, and comes with a significant performance cost.

If possible, avoid placeholders in these directives.

## `resolve_root_symlink`

By default, if the document root is a symbolic link, it is automatically resolved by FrankenPHP (this is needed by PHP).
If the document root is not a symlink, you can disable this feature.

```caddyfile
php_server {
    resolve_root_symlink false
}
```

This will improve performance if the `root` directive contains [placeholders](https://caddyserver.com/docs/conventions#placeholders). The gain will be negilible in other cases.

## PHP Performance

FrankenPHP executes the official PHP interpreter.
All usual PHP-related performance optimizations apply with FrankenPHP.

In particular:

* check that [OPcache](https://www.php.net/manual/en/book.opcache.php) is installed, enabled and properly configured
* enable [Composer autoloader optimizations](https://getcomposer.org/doc/articles/autoloader-optimization.md)
* ensure that the `realpath` cache is big enough for the needs of your application
* use [preloading](https://www.php.net/manual/en/opcache.preloading.php)

For more details, read [the dedicated Symfony documentation entry](https://symfony.com/doc/current/performance.html)
(most tips are useful even if you don't use Symfony).
