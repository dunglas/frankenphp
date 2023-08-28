# Create a Static Build

Instead of using a local installation of the PHP library,
it's possible to create a static build of FrankenPHP thanks to the great [static-php-cli project](https://github.com/crazywhalecc/static-php-cli) (despite its name, this project support all SAPIs, not only CLI).

With this method, a single, portable, binary will contain the PHP interpreter, the Caddy web server and FrankenPHP!

## Linux

We provide a Docker image to build a Linux static binary:

```console
docker buildx bake --load static-builder
docker cp $(docker create --name static-builder dunglas/frankenphp:static-builder):/go/src/app/caddy/frankenphp/frankenphp frankenphp ; docker rm static-builder
```

The resulting static binary is named `frankenphp` and is available in the current directory.

If you want to build the static binary without Docker, take a look to the `static-builder.Dockerfile` file.

### Custom Extensions

By default, most popular PHP extensions are compiled.

To reduce the size of the binary and to reduce the attack surface, you can choose the list of extensions to build using the `PHP_EXTENSIONS` Docker ARG.

For instance, run the following command to only build the `opcache` extension:

```console
docker buildx bake --load --set static-builder.args.PHP_EXTENSIONS=opcache static-builder
# ...
```

### GitHub Token

If you hit the GitHub API rate limit, set a GitHub Personal Access Token in an environment variable named `GITHUB_TOKEN`:

```console
GITHUB_TOKEN="xxx" docker --load buildx bake static-builder
# ...
```
