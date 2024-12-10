# Create a Static Build

Instead of using a local installation of the PHP library,
it's possible to create a static build of FrankenPHP thanks to the great [static-php-cli project](https://github.com/crazywhalecc/static-php-cli) (despite its name, this project support all SAPIs, not only CLI).

With this method, a single, portable, binary will contain the PHP interpreter, the Caddy web server and FrankenPHP!

FrankenPHP also supports [embedding the PHP app in the static binary](embed.md).

## Linux

We provide a Docker image to build a Linux static binary:

```console
docker buildx bake --load static-builder
docker cp $(docker create --name static-builder dunglas/frankenphp:static-builder):/go/src/app/dist/frankenphp-linux-$(uname -m) frankenphp ; docker rm static-builder
```

The resulting static binary is named `frankenphp` and is available in the current directory.

If you want to build the static binary without Docker, take a look at the macOS instructions, which also works for Linux.

### Custom Extensions

By default, most popular PHP extensions are compiled.

To reduce the size of the binary and to reduce the attack surface, you can choose the list of extensions to build using the `PHP_EXTENSIONS` Docker ARG.

For instance, run the following command to only build the `opcache` extension:

```console
docker buildx bake --load --set static-builder.args.PHP_EXTENSIONS=opcache,pdo_sqlite static-builder
# ...
```

To add libraries enabling additional functionality to the extensions you've enabled, you can pass use the `PHP_EXTENSION_LIBS` Docker ARG:

```console
docker buildx bake \
  --load \
  --set static-builder.args.PHP_EXTENSIONS=gd \
  --set static-builder.args.PHP_EXTENSION_LIBS=libjpeg,libwebp \
  static-builder
```

### Extra Caddy Modules

To add extra Caddy modules or pass other arguments to [xcaddy](https://github.com/caddyserver/xcaddy), use the `XCADDY_ARGS` Docker ARG:

```console
docker buildx bake \
  --load \
  --set static-builder.args.XCADDY_ARGS="--with github.com/darkweak/souin/plugins/caddy --with github.com/dunglas/mercure/caddy --with github.com/dunglas/vulcain/caddy" \
  static-builder
```

In this example, we add the [Souin](https://souin.io) HTTP cache module for Caddy as well as the [Mercure](https://mercure.rocks) and [Vulcain](https://vulcain.rocks) modules.

> [!TIP]
>
> The Mercure and Vulcain modules are included by default if `XCADDY_ARGS` is empty or not set.
> If you customize the value of `XCADDY_ARGS`, you must include them explicitly if you want them to be included.

See also how to [customize the build](#customizing-the-build)

### GitHub Token

If you hit the GitHub API rate limit, set a GitHub Personal Access Token in an environment variable named `GITHUB_TOKEN`:

```console
GITHUB_TOKEN="xxx" docker --load buildx bake static-builder
# ...
```

## macOS

Run the following script to create a static binary for macOS (you must have [Homebrew](https://brew.sh/) installed):

```console
git clone https://github.com/dunglas/frankenphp
cd frankenphp
./build-static.sh
```

Note: this script also works on Linux (and probably on other Unixes), and is used internally by the Docker based static builder we provide.

## Customizing The Build

The following environment variables can be passed to `docker build` and to the `build-static.sh`
script to customize the static build:

* `FRANKENPHP_VERSION`: the version of FrankenPHP to use
* `PHP_VERSION`: the version of PHP to use
* `PHP_EXTENSIONS`: the PHP extensions to build ([list of supported extensions](https://static-php.dev/en/guide/extensions.html))
* `PHP_EXTENSION_LIBS`: extra libraries to build that add features to the extensions
* `XCADDY_ARGS`: arguments to pass to [xcaddy](https://github.com/caddyserver/xcaddy), for instance to add extra Caddy modules
* `EMBED`: path of the PHP application to embed in the binary
* `CLEAN`: when set, libphp and all its dependencies are built from scratch (no cache)
* `NO_COMPRESS`: don't compress the resulting binary using UPX
* `DEBUG_SYMBOLS`: when set, debug-symbols will not be stripped and will be added within the binary
* `MIMALLOC`: (experimental, Linux-only) replace musl's mallocng by [mimalloc](https://github.com/microsoft/mimalloc) for improved performance
* `RELEASE`: (maintainers only) when set, the resulting binary will be uploaded on GitHub
