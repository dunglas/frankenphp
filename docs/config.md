# Configuration

FrankenPHP, Caddy as well the Mercure and Vulcain modules can be configured using [the formats supported by Caddy](https://caddyserver.com/docs/getting-started#your-first-config).

In the Docker image, the `Caddyfile` is located at `/etc/caddy/Caddyfile`.

You can also configure PHP using `php.ini` as usual.

In the Docker image, the `php.ini` file is not present, you can create it or `COPY` manually.

If you copy `php.ini` from `$PHP_INI_DIR/php.ini-production` or `$PHP_INI_DIR/php.ini-development` you also must set variable `variables_order = "EGPCS"`, because default value for `variables_order` is `"EGPCS"` but in `php.ini-production` and `php.ini-development` we have `"GPCS"`. And in this case `worker` not work propertly.

```dockerfile
FROM dunglas/frankenphp

RUN cp $PHP_INI_DIR/php.ini-production $PHP_INI_DIR/php.ini; \
    sed -i 's/variables_order = "GPCS"/variables_order = "EGPCS"/' $PHP_INI_DIR/php.ini;
```

## Caddy Directives

To register the FrankenPHP executor, the `frankenphp` directive must be set in Caddy global options, then the `php_server` or the `php` HTTP directives must be set under routes serving PHP scripts.

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
    encode zstd gzip
    # Execute PHP files in the current directory and serve assets
    php_server
}
```

Optionally, the number of threads to create and [worker scripts](worker.md) to start with the server can be specified under the global directive.

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

Alternatively, the short form of the `worker` directive can also be used:

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
    root /path/to/app/public/
}


other.example.com {
    root /path/to/other/public/
}
...
```

Using the `php_server` directive is generaly what you need,
but if you need full control, you can use the lower level `php` directive:

Using the `php_server` directive is equivalent to this configuration:

```caddyfile
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
```

The `php_server` and the `php` directives have the following options:

```caddyfile
php_server [<matcher>] {
    root <directory> # Sets the root folder to the site. Default: `root` directive.
    split_path <delim...> # Sets the substrings for splitting the URI into two parts. The first matching substring will be used to split the "path info" from the path. The first piece is suffixed with the matching substring and will be assumed as the actual resource (CGI script) name. The second piece will be set to PATH_INFO for the CGI script to use. Default: `.php`
    resolve_root_symlink # Enables resolving the `root` directory to its actual value by evaluating a symbolic link, if one exists.
    env <key> <value> # Sets an extra environment variable to the given value. Can be specified more than once for multiple environment variables.
}
```

## Environment Variables

The following environment variables can be used to inject Caddy directives in the `Caddyfile` without modifying it:

* `SERVER_NAME` change the server name
* `CADDY_GLOBAL_OPTIONS`: inject [global options](https://caddyserver.com/docs/caddyfile/options)
* `FRANKENPHP_CONFIG`: inject config under the `frankenphp` directive

Unlike with FPM and CLI SAPIs, environment variables are **not** exposed by default in superglobals `$_SERVER` and `$_ENV`.

To propagate environment variables to `$_SERVER` and `$_ENV`, set the `php.ini` `variables_order` directive to `EGPS`.

## Enable the Debug Mode

When using the Docker image, set the `CADDY_DEBUG` environment variable to `debug` to enable the debug mode:

```console
docker run -v $PWD:/app/public \
    -e CADDY_GLOBAL_OPTIONS=debug \
    -p 80:80 -p 443:443 \
    dunglas/frankenphp
```
