# Configuration

FrankenPHP, Caddy as well as the Mercure and Vulcain modules can be configured using [the formats supported by Caddy](https://caddyserver.com/docs/getting-started#your-first-config).

In the Docker image, the `Caddyfile` is located at `/etc/caddy/Caddyfile`.

You can also configure PHP using `php.ini` as usual.

In the Docker image, the `php.ini` file is not present, you can create it manually  or copy an official template:

```dockerfile
FROM dunglas/frankenphp

# Developement:
RUN cp $PHP_INI_DIR/php.ini-development $PHP_INI_DIR/php.ini

# Or production:
RUN cp $PHP_INI_DIR/php.ini-production $PHP_INI_DIR/php.ini
```

## Caddyfile Config

To register the FrankenPHP executor, the `frankenphp` [global option](https://caddyserver.com/docs/caddyfile/concepts#global-options) must be set, then the `php_server` or the `php` [HTTP directives](https://caddyserver.com/docs/caddyfile/concepts#directives) may be used within the site blocks to serve your PHP app.

Minimal example:

```caddyfile
{
	# Enable FrankenPHP
	frankenphp
	# Configure when the directive must be executed
	order php_server before file_server
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
}
```

### Full Duplex (HTTP/1)

When using HTTP/1.x, it may be desirable to enable full-duplex mode to allow writing a response before the entire body
has been read. (for example: websockets, SSE, etc.)

This is an opt-in configuration that needs to be added to the global options in the caddyfile:

```caddyfile
{
  servers {
    enable_full_duplex
  }
}
```

This can also be configured using the `CADDY_GLOBAL_OPTIONS` environment config:

```
ENV CADDY_GLOBAL_OPTIONS="servers { enable_full_duplex }"
```

You can find more information about this setting in Caddy documentation: 
https://caddyserver.com/docs/caddyfile/options#enable-full-duplex

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
