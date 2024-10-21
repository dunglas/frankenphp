# Configuration

FrankenPHP, Caddy as well as the Mercure and Vulcain modules can be configured using [the formats supported by Caddy](https://caddyserver.com/docs/getting-started#your-first-config).

In [the Docker images](docker.md), the `Caddyfile` is located at `/etc/caddy/Caddyfile`.
The static binary will look for the `Caddyfile` in the directory in which it is started.

PHP itself can be configured [using a `php.ini` file](https://www.php.net/manual/en/configuration.file.php).

By default, PHP supplied with Docker images and the one included in the static binary will look for a `php.ini` file in the directory where FrankenPHP is started and in `/usr/local/etc/php/`. They will also load all files ending in `.ini` from `/usr/local/etc/php/conf.d/`.

No `php.ini` file is present by default, you should copy an official template provided by the PHP project.

On Docker, the templates are provided in the images:

```dockerfile
FROM dunglas/frankenphp

# Production:
RUN cp $PHP_INI_DIR/php.ini-production $PHP_INI_DIR/php.ini

# Or developement:
RUN cp $PHP_INI_DIR/php.ini-development $PHP_INI_DIR/php.ini
```

If you don't use Docker, copy one of `php.ini-production` or `php.ini-development` provided [in the PHP sources](https://github.com/php/php-src/).

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
		worker {
			file <path> # Sets the path to the worker script.
			num <num> # Sets the number of PHP threads to start, defaults to 2x the number of available CPUs.
			env <key> <value> # Sets an extra environment variable to the given value. Can be specified more than once for multiple environment variables.
			watch <path> # Sets the path to watch for file changes. Can be specified more than once for multiple paths.
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

You can also define multiple workers if you serve multiple apps on the same server:

```caddyfile
{
	frankenphp {
		worker /path/to/app/public/index.php <num>
		worker /path/to/other/public/index.php <num>
	}
}

app.example.com {
	root * /path/to/app/public
	php_server
}

other.example.com {
	root * /path/to/other/public
	php_server
}

# ...
```

Using the `php_server` directive is generally what you need,
but if you need full control, you can use the lower level `php` directive:

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

* The `**` pattern signifies recursive watching
* Directories can also be relative (to where the FrankenPHP process is started from)
* If you have multiple workers defined, all of them will be restarted when a file changes
* Be wary about watching files that are created at runtime (like logs) since they might cause unwanted worker restarts.

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
CADDY_GLOBAL_OPTIONS="servers { enable_full_duplex }"
```

You can find more information about this setting in the [Caddy documentation](https://caddyserver.com/docs/caddyfile/options#enable-full-duplex).

## Environment Variables

The following environment variables can be used to inject Caddy directives in the `Caddyfile` without modifying it:

* `SERVER_NAME`: change [the addresses on which to listen](https://caddyserver.com/docs/caddyfile/concepts#addresses), the provided hostnames will also be used for the generated TLS certificate
* `CADDY_GLOBAL_OPTIONS`: inject [global options](https://caddyserver.com/docs/caddyfile/options)
* `FRANKENPHP_CONFIG`: inject config under the `frankenphp` directive

As for FPM and CLI SAPIs, environment variables are exposed by default in the `$_SERVER` superglobal.

The `S` value of [the `variables_order` PHP directive](https://www.php.net/manual/en/ini.core.php#ini.variables-order) is always equivalent to `ES` regardless of the placement of `E` elsewhere in this directive.

## PHP config

To load [additional PHP configuration files](https://www.php.net/manual/en/configuration.file.php#configuration.file.scan),
the `PHP_INI_SCAN_DIR` environment variable can be used.
When set, PHP will load all the file with the `.ini` extension present in the given directories.

## Enable the Debug Mode

When using the Docker image, set the `CADDY_GLOBAL_OPTIONS` environment variable to `debug` to enable the debug mode:

```console
docker run -v $PWD:/app/public \
    -e CADDY_GLOBAL_OPTIONS=debug \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```
