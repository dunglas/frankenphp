# Real-time

FrankenPHP comes with a built-in [Mercure](https://mercure.rocks) hub!
Mercure allows you to push real-time events to all the connected devices: they will receive a JavaScript event instantly.

No JS library or SDK is required!

![Mercure](mercure-hub.png)

To enable the Mercure hub, update the `Caddyfile` as described [on Mercure's site](https://mercure.rocks/docs/hub/config).

The path of the Mercure hub is `/.well-known/mercure`.
When running FrankenPHP inside Docker, the full send URL would look like `http://php/.well-known/mercure` (with `php` being the container's name running FrankenPHP).

To push Mercure updates from your code, we recommend the [Symfony Mercure Component](https://symfony.com/components/Mercure) (you don't need the Symfony full-stack framework to use it).
