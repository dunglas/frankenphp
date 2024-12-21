# Using Classic Mode

Without any additional configuration, FrankenPHP operates in classic mode. In this mode, FrankenPHP functions like a traditional PHP server, directly serving PHP files. This makes it a seamless drop-in replacement for PHP-FPM or Apache with mod_php.

Similar to Caddy, FrankenPHP accepts an unlimited number of connections and uses a [fixed number of threads](config.md#caddyfile-config) to serve them. The number of accepted and queued connections is limited only by the available system resources. The PHP thread pool operates with a fixed number of threads initialized at startup, comparable to the static mode of PHP-FPM.

Queued connections will wait indefinitely until a PHP thread is available to serve them. To prevent that, set a reasonable [write timeout in Caddy](https://caddyserver.com/docs/caddyfile/options#timeouts).

Each Caddy instance will only spin up one FrankenPHP thread pool, which will be shared across all `php_server` blocks.
