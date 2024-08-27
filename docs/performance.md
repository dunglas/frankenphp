# Performance

By default, FrankenPHP tries to offer a good compromise between performance and ease of use.
However, it is possible to substantially improve performance using an appropriate configuration.

## Number of Threads and Workers

By default, FrankenPHP starts 2 times more threads and workers (in worker mode) than the available numbers of CPU.

The appropriate values depend heavily on how your application is written, what it does and your hardware.
We strongly recommend changing these values.

To find the right values, it's best to run load tests simulating real traffic.
[k6](https://k6.io) and [Gatling](https://gatling.io) are good tools for this.

To configure the number of threads, use the `num_threads` option of the `php_server` and `php` directives.
To change the number of workers, use the `num` option of the `worker` section of the `frankenphp` directive.

## Worker Mode

Enabling [the worker mode](worker.md) dramatically improves performance,
but your app must be adapted to be compatible with this mode:
you need to create a worker script and to be sure that the app is not leaking memory.

## Don't Use musl

The static binaries we provide and the Alpine Linux variant of the official Docker images
are using [the musl libc](https://musl.libc.org).

PHP is known to be significantly slower when using this alternative C library instead of the traditional GNU library,
especially when compiled in ZTS mode (thread-safe), which is required for FrankenPHP.

Also, some bugs also only happen when using musl.

In production environements, we strongly recommend to use the glibc.

This can be achieved by using the Debian Docker images (the default) and [by compiling FrankenPHP from sources](compile.md).

Alternatively, we provide static binaries compiled with [the mimalloc allocator](https://github.com/microsoft/mimalloc), which makes FrankenPHP+musl faster (but still slower than FrankenPHP+glibc).

## Go Runtime Configuration

FrankenPHP is written in Go.

In general, the Go runtime doesn't require any special configuration, but in certain circumstances,
specific configuration improves performance.

You likely want to set the `GODEBUG` environment variable to `cgocheck=0` (the default in the FrankenPHP Docker images).

If you run FrankenPHP in containers (Docker, Kubernetes, LXC...) and limit the memory available for the containers,
set the `GOMEMLIMIT` environment variable to the available amount of memory.

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
However, this prevents caching these values, and comes with a significant performance cost.

If possible, avoid placeholders in these directives.

## `resolve_root_symlink`

By default, if the document root is a symbolic link, it is automatically resolved by FrankenPHP (this is necessary for PHP to work properly).
If the document root is not a symlink, you can disable this feature.

```caddyfile
php_server {
    resolve_root_symlink false
}
```

This will improve performance if the `root` directive contains [placeholders](https://caddyserver.com/docs/conventions#placeholders).
The gain will be negligible in other cases.

## Logs

Logging is obviously very useful, but, by definition,
it requires I/O operations and memory allocations, which considerably reduces performance.
Make sure you [set the logging level](https://caddyserver.com/docs/caddyfile/options#log) correctly,
and only log what's necessary.

## PHP Performance

FrankenPHP uses the official PHP interpreter.
All usual PHP-related performance optimizations apply with FrankenPHP.

In particular:

* check that [OPcache](https://www.php.net/manual/en/book.opcache.php) is installed, enabled and properly configured
* enable [Composer autoloader optimizations](https://getcomposer.org/doc/articles/autoloader-optimization.md)
* ensure that the `realpath` cache is big enough for the needs of your application
* use [preloading](https://www.php.net/manual/en/opcache.preloading.php)

For more details, read [the dedicated Symfony documentation entry](https://symfony.com/doc/current/performance.html)
(most tips are useful even if you don't use Symfony).
