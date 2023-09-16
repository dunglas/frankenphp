# Configuration

FrankenPHP, Caddy as well the Mercure and Vulcain modules can be configured using [the formats supported by Caddy](https://caddyserver.com/docs/getting-started#your-first-config).

In the Docker image, the `Caddyfile` is located at `/etc/Caddyfile`.

You can also configure PHP using `php.ini` as usual.

In the Docker image, the `php.ini` file is located at `/usr/local/lib/php.ini`.

## Caddy Directives

To register the FrankenPHP executor, the `frankenphp` directive must be set in Caddy global options, then the `php` HTTP directive must be set under routes serving PHP scripts:


Then, you can use the `php` HTTP directive to execute PHP scripts:

```caddyfile
{
    frankenphp
}

localhost {
    route {
        php {
            root <directory> # Sets the root folder to the site. Default: `root` directive.
            split_path <delim...> # Sets the substrings for splitting the URI into two parts. The first matching substring will be used to split the "path info" from the path. The first piece is suffixed with the matching substring and will be assumed as the actual resource (CGI script) name. The second piece will be set to PATH_INFO for the CGI script to use. Default: `.php`
            resolve_root_symlink # Enables resolving the `root` directory to its actual value by evaluating a symbolic link, if one exists.
            env <key> <value> # Sets an extra environment variable to the given value. Can be specified more than once for multiple environment variables.
        }
    }
}
```

Optionnaly, the number of threads to create and [worker scripts](worker.md) to start with the server can be specified under the global directive.

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

## Environment Variables

The following environment variables can be used to inject Caddy directives in the `Caddyfile` without modifying it:

* `SERVER_NAME` change the server name
* `CADDY_GLOBAL_OPTIONS`: inject [global options](https://caddyserver.com/docs/caddyfile/options)
* `FRANKENPHP_CONFIG`: inject config under the `frankenphp` directive

Unlike with FPM and CLI SAPIs, environment variables are **not** exposed by default in superglobals `$_SERVER` and `$_ENV`.

To propagate environment variables to `$_SERVER` and `$_ENV`, set the `php.ini` `variables_order` directive to `EGPS`.

## Enable the Debug Mode

When using the Docker image, set the `CADDY_DEBUG` environment variable to `debug` to enable the debug mode:

```
docker run -v $PWD:/app/public \
    -e CADDY_GLOBAL_OPTIONS=debug \
    -p 80:80 -p 443:443 \
    dunglas/frankenphp
```
