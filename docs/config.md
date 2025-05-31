# Configuration

FrankenPHP, Caddy as well as the Mercure and Vulcain modules can be configured using [the formats supported by Caddy](https://caddyserver.com/docs/getting-started#your-first-config).

In [the Docker images](docker.md), the `Caddyfile` is located at `/etc/frankenphp/Caddyfile`.
The static binary will also look for the `Caddyfile` in the directory where the `frankenphp run` command is executed.
You can specify a custom path with the `-c` or `--config` option.

PHP itself can be configured [using a `php.ini` file](https://www.php.net/manual/en/configuration.file.php).

Depending on your installation method, the PHP interpreter will look for configuration files in locations described above.

## Docker

- `php.ini`: `/usr/local/etc/php/php.ini` (no `php.ini` is provided by default)
- additional configuration files: `/usr/local/etc/php/conf.d/*.ini`
- PHP extensions: `/usr/local/lib/php/extensions/no-debug-zts-<YYYYMMDD>/`
- You should copy an official template provided by the PHP project:

```dockerfile
FROM dunglas/frankenphp

# Production:
RUN cp $PHP_INI_DIR/php.ini-production $PHP_INI_DIR/php.ini

# Or development:
RUN cp $PHP_INI_DIR/php.ini-development $PHP_INI_DIR/php.ini
```

## RPM and Debian packages

- `php.ini`: `/etc/frankenphp/php.ini` (a `php.ini` file with production presets is provided by default)
- additional configuration files: `/etc/frankenphp/php.d/*.ini`
- PHP extensions: `/usr/lib/frankenphp/modules/`

## Static binary

- `php.ini`: The directory in which `frankenphp run` or `frankenphp php-server` is executed, then `/etc/frankenphp/php.ini`
- additional configuration files: `/etc/frankenphp/php.d/*.ini`
- PHP extensions: cannot be loaded, bundle them in the binary itself
- copy one of `php.ini-production` or `php.ini-development` provided [in the PHP sources](https://github.com/php/php-src/).

## Caddyfile Config

To register the FrankenPHP executor, the `frankenphp` [global option](https://caddyserver.com/docs/caddyfile/concepts#global-options) must be set, then the `php_server` or the `php` [HTTP directives](https://caddyserver.com/docs/caddyfile/concepts#directives) may be used within the site blocks to serve your PHP app.

Minimal example:

```caddyfile
{
	# Enable FrankenPHP
	frankenphp
}

localhost {
	# Enable compression (optional)
	encode zstd br gzip
	# Execute PHP files in the current directory and serve assets
	php_server
}
```

Optionally, the number of threads to create and [worker scripts](worker.md) to start with the server can be specified under the global option.

```caddyfile
{
	frankenphp {
		num_threads <num_threads> # Sets the number of PHP threads to start. Default: 2x the number of available CPUs.
		max_threads <num_threads> # Limits the number of additional PHP threads that can be started at runtime. Default: num_threads. Can be set to 'auto'.
		max_wait_time <duration> # Sets the maximum time a request may wait for a free PHP thread before timing out. Default: disabled.
		php_ini <key> <value> # Set a php.ini directive. Can be used several times to set multiple directives.
		worker {
			file <path> # Sets the path to the worker script.
			num <num> # Sets the number of PHP threads to start, defaults to 2x the number of available CPUs.
			env <key> <value> # Sets an extra environment variable to the given value. Can be specified more than once for multiple environment variables.
			watch <path> # Sets the path to watch for file changes. Can be specified more than once for multiple paths.
			name <name> # Sets the name of the worker, used in logs and metrics. Default: absolute path of worker file
		}
	}
}

# ...
```

Alternatively, you may use the one-line short form of the `worker` option:

```caddyfile
{
	frankenphp {
		worker <file> <num>
	}
}

# ...
```

Worker blocks can also be defined inside a `php` or `php_server` block. In this case, the worker inherits environment variables and root path from the parent directive and is only accessible by that specific domain:

```caddyfile
{
	frankenphp
}
example.com {
	root /path/to/app
	php_server {
		root <path>
		worker {
			file <path, can be relative to root>
			num <num>
			env <key> <value>
			watch <path>
			name <name>
		}
	}
}
```

You can also define multiple workers if you serve multiple apps on the same server:

```caddyfile
{
	frankenphp {
		worker /path/to/app/public/index.php <num>
		worker {
			file /path/to/other/public/index.php
			num <num>
			env APP_ENV dev
		}
	}
}

app.example.com {
	root /path/to/app/public
	php_server
}

other.example.com {
	root /path/to/other/public
	php_server {
		env APP_ENV dev
	}
}

# ...
```

Is equivalent to

```caddyfile
{
	frankenphp
}

app.example.com {
	php_server {
		root /path/to/app/public
		worker index.php <num>
	}
}

other.example.com {
	php_server {
		root /path/to/other/public
		env APP_ENV dev
		worker index.php <num>
	}
}

# ...
```

Using the `php_server` directive is generally what you need,
but if you need full control, you can use the lower-level `php` directive.
The `php` directive passes all input to PHP, instead of first checking whether
it's a PHP file or not. Read more about it in the [performance page](performance.md).

Using the `php_server` directive is equivalent to this configuration:

```caddyfile
route {
	# Add trailing slash for directory requests
	@canonicalPath {
		file {path}/index.php
		not path */
	}
	redir @canonicalPath {path}/ 308
	# If the requested file does not exist, try index files
	@indexFiles file {
		try_files {path} {path}/index.php index.php
		split_path .php
	}
	rewrite @indexFiles {http.matchers.file.relative}
	# FrankenPHP!
	@phpFiles path *.php
	php @phpFiles
	file_server
}
```

The `php_server` and the `php` directives have the following options:

```caddyfile
php_server [<matcher>] {
	root <directory> # Sets the root folder to the site. Default: `root` directive.
	split_path <delim...> # Sets the substrings for splitting the URI into two parts. The first matching substring will be used to split the "path info" from the path. The first piece is suffixed with the matching substring and will be assumed as the actual resource (CGI script) name. The second piece will be set to PATH_INFO for the script to use. Default: `.php`
	resolve_root_symlink false # Disables resolving the `root` directory to its actual value by evaluating a symbolic link, if one exists (enabled by default).
	env <key> <value> # Sets an extra environment variable to the given value. Can be specified more than once for multiple environment variables.
	file_server off # Disables the built-in file_server directive.
}
```

### Watching for File Changes

Since workers only boot your application once and keep it in memory, any changes
to your PHP files will not be reflected immediately.

Workers can instead be restarted on file changes via the `watch` directive.
This is useful for development environments.

```caddyfile
{
	frankenphp {
		worker {
			file  /path/to/app/public/worker.php
			watch
		}
	}
}
```

If the `watch` directory is not specified, it will fall back to `./**/*.{php,yaml,yml,twig,env}`,
which watches all `.php`, `.yaml`, `.yml`, `.twig` and `.env` files in the directory and subdirectories
where the FrankenPHP process was started. You can instead also specify one or more directories via a
[shell filename pattern](https://pkg.go.dev/path/filepath#Match):

```caddyfile
{
	frankenphp {
		worker {
			file  /path/to/app/public/worker.php
			watch /path/to/app # watches all files in all subdirectories of /path/to/app
			watch /path/to/app/*.php # watches files ending in .php in /path/to/app
			watch /path/to/app/**/*.php # watches PHP files in /path/to/app and subdirectories
			watch /path/to/app/**/*.{php,twig} # watches PHP and Twig files in /path/to/app and subdirectories
		}
	}
}
```

- The `**` pattern signifies recursive watching
- Directories can also be relative (to where the FrankenPHP process is started from)
- If you have multiple workers defined, all of them will be restarted when a file changes
- Be wary about watching files that are created at runtime (like logs) since they might cause unwanted worker restarts.

The file watcher is based on [e-dant/watcher](https://github.com/e-dant/watcher).

### Full Duplex (HTTP/1)

When using HTTP/1.x, it may be desirable to enable full-duplex mode to allow writing a response before the entire body
has been read. (for example: WebSocket, Server-Sent Events, etc.)

This is an opt-in configuration that needs to be added to the global options in the `Caddyfile`:

```caddyfile
{
  servers {
	enable_full_duplex
  }
}
```

> [!CAUTION]
>
> Enabling this option may cause old HTTP/1.x clients that don't support full-duplex to deadlock.
> This can also be configured using the `CADDY_GLOBAL_OPTIONS` environment config:

```sh
CADDY_GLOBAL_OPTIONS="servers {
  enable_full_duplex
}"
```

You can find more information about this setting in the [Caddy documentation](https://caddyserver.com/docs/caddyfile/options#enable-full-duplex).

## Environment Variables

The following environment variables can be used to inject Caddy directives in the `Caddyfile` without modifying it:

- `SERVER_NAME`: change [the addresses on which to listen](https://caddyserver.com/docs/caddyfile/concepts#addresses), the provided hostnames will also be used for the generated TLS certificate
- `CADDY_GLOBAL_OPTIONS`: inject [global options](https://caddyserver.com/docs/caddyfile/options)
- `FRANKENPHP_CONFIG`: inject config under the `frankenphp` directive

As for FPM and CLI SAPIs, environment variables are exposed by default in the `$_SERVER` superglobal.

The `S` value of [the `variables_order` PHP directive](https://www.php.net/manual/en/ini.core.php#ini.variables-order) is always equivalent to `ES` regardless of the placement of `E` elsewhere in this directive.

## PHP config

To load [additional PHP configuration files](https://www.php.net/manual/en/configuration.file.php#configuration.file.scan),
the `PHP_INI_SCAN_DIR` environment variable can be used.
When set, PHP will load all the file with the `.ini` extension present in the given directories.

You can also change the PHP configuration using the `php_ini` directive in the `Caddyfile`:

```caddyfile
{
	frankenphp {
		php_ini memory_limit 256M

		# or

		php_ini {
			memory_limit 256M
			max_execution_time 15
		}
	}
}
```

## The php-server Command

The `php-server` command is a convenient way to start a production-ready PHP server. It's especially useful for quick deployments, demos, development, or to run an [embedded app](embed.md).

```console
frankenphp php-server [--domain <example.com>] [--root <path>] [--listen <addr>] [--worker /path/to/worker.php<,nb-workers>] [--watch <paths...>] [--access-log] [--debug] [--no-compress] [--mercure]
```

### Options

- `--domain`, `-d`: Domain name at which to serve the files. If specified, the server will use HTTPS and automatically obtain a Let's Encrypt certificate.
- `--root`, `-r`: The path to the root of the site. If not specified and using an embedded app, it will use the embedded_app/public directory by default.
- `--listen`, `-l`: The address to which to bind the listener. Default is `:80` or `:443` if a domain is specified.
- `--worker`, `-w`: Worker script to run. Can be specified multiple times for multiple workers.
- `--watch`: Directory to watch for file changes. Can be specified multiple times for multiple directories.
- `--access-log`, `-a`: Enable the access log.
- `--debug`, `-v`: Enable verbose debug logs.
- `--mercure`, `-m`: Enable the built-in Mercure.rocks hub.
- `--no-compress`: Disable Zstandard, Brotli and Gzip compression.

### Examples

Start a server with the current directory as the document root:

```console
frankenphp php-server --root ./
```

Start a server with HTTPS enabled:

```console
frankenphp php-server --domain example.com
```

Start a server with a worker:

```console
frankenphp php-server --worker public/index.php
```

## Enable the Debug Mode

When using the Docker image, set the `CADDY_GLOBAL_OPTIONS` environment variable to `debug` to enable the debug mode:

```console
docker run -v $PWD:/app/public \
    -e CADDY_GLOBAL_OPTIONS=debug \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```
