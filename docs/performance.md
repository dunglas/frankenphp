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

### Rule of Thumb—Threads

A good starting point is to set the number of threads to the number of CPU cores * 2, however, this gets complicated
when using orchestrations like Kubernetes or cgroups, in general.

However, if your application is heavily i/o bound,
you can usually increase the number of threads to a much higher number, assuming you have enough memory.
This will largely depend on your application and the hardware/orchestration system you are running on.
You may need to experiment with different values to find the optimal number.

#### Cores

FrankenPHP is a multithreaded application,
and as such,
running it on less than 2 cores for production workloads is [not recommended](https://github.com/dunglas/frankenphp/discussions/941#discussioncomment-10195431).

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

## Load Balancing

It is recommended to use caddy to load balance between multiple instances of FrankenPHP to ensure high availability,
performance and reliability.
Nginx can also be used,
but it is [not nearly as performant as Caddy](https://www.patrickdap.com/post/benchmarking-is-hard/) without professional tuning.

## Go Runtime Configuration

FrankenPHP is written in Go.

In general, the Go runtime doesn't require any special configuration, but in certain circumstances,
specific configuration improves performance.

You likely want to set the `GODEBUG` environment variable to `cgocheck=0` (the default in the FrankenPHP Docker images).

If you run FrankenPHP in containers (Docker, Kubernetes, LXC...) and limit the memory available for the containers,
set the `GOMEMLIMIT` environment variable to the available amount of memory.

For more details, [the Go documentation page dedicated to this subject](https://pkg.go.dev/runtime#hdr-Environment_Variables) is a must-read to get the most out of the runtime.

### Rule of Thumb—Memory

When dealing with memory limits (such as in containers),
this memory limit isn’t shared with the application and thus must be set manually; just like with any other workload.

If you are using Kubernetes, it is usually recommended to set GOMEMLIMIT like so:

```yaml
env:
- name: GOMEMLIMIT
  valueFrom:
    resourceFieldRef:
      resource: limits.memory
```

However, PHP also uses memory separate from the go runtime using the `memory_limit` directive in `php.ini`.
Thus, a balance is required between the Go runtime and the PHP runtime.

If your application is using the entire available memory (i.e., no memory limit),
then you can basically skip the remainder of this section,
as the Go runtime will automatically adjust to the available memory.
However, if you are using container memory limits,
you will need to adjust the Go runtime memory limit to ensure you don’t crash from an out-of-memory error.

A quick calculation you can use to determine the optimal memory limit for the Go runtime is:

```
POD_MEMORY_LIMIT = (PHP_MAX_MEMORY * NUM_THREADS) + (NUM_CPU * 100MB)
GOMEMLIMIT = (NUM_CPU * 1000000000)
```

The 100MB is an extremely conservative estimate of the memory
required for the Go runtime to run FrankenPHP and terminate SSL.
This value may need to be adjusted based on your specific application.

Thus, if you have 16 threads on an 8-core machine with a 128mb memory limit for PHP,
then you should allocate ~2.9GB to the container and set `GOMEMLIMIT` to 0.8GB (`8000000000`).

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
