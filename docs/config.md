# Configuration

FrankenPHP, Caddy as well the Mercure and Vulcain modules can be configured using [the formats supported by Caddy](https://caddyserver.com/docs/getting-started#your-first-config).

In the Docker image, the `Caddyfile` is located at `/etc/Caddyfile`.

You can also configure PHP using `php.ini` as usual.

In the Docker image, the `php.ini` file is located at `/usr/local/lib/php.ini`.


## Environment Variables

The following environment variables can be used to inject Caddy directives in the `Caddyfile` without modifying it:

* `SERVER_NAME` change the server name
* `CADDY_GLOBAL_OPTIONS`: inject [global options](https://caddyserver.com/docs/caddyfile/options)
* `FRANKENPHP_CONFIG`: inject config under the `frankenphp` directive

## Enable the Debug Mode

When using the Docker image, set the `CADDY_DEBUG` environment variable to `debug` to enable the debug mode:

```
docker run -v $PWD:/app/public \
    -e CADDY_GLOBAL_OPTIONS=debug \
    -p 80:80 -p 443:443 \
    dunglas/frankenphp
```
