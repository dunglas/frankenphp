# Configuration

FrankenPHP, Caddy as well the Mercure and Vulcain modules can be configured using [the formats supported by Caddy](https://caddyserver.com/docs/getting-started#your-first-config).

In the Docker image, the `Caddyfile` is located at `/etc/Caddyfile`.

You can also configure PHP using `php.ini` as usual.

In the Docker image, the `php.ini` file is located at `/usr/local/lib/php.ini`.

## Enable the Debug Mode

When using the Docker image, set the `CADDY_DEBUG` environment variable to `debug` to enable the debug mode:

```
docker run -v $PWD:/app/public \
    -e CADDY_DEBUG=debug \
    -p 80:80 -p 443:443 \
    dunglas/frankenphp
```
