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
docker buildx bake --load --set static-builder.args.PHP_EXTENSIONS=opcache,pdo_sqlite static-builder
# ...
```

See [the list of supported extensions](https://static-php-cli.zhamao.me/en/guide/extensions.html).

### GitHub Token

If you hit the GitHub API rate limit, set a GitHub Personal Access Token in an environment variable named `GITHUB_TOKEN`:

```console
GITHUB_TOKEN="xxx" docker --load buildx bake static-builder
# ...
```

## macOS

Note: only a very limited subset of extensions are currently available for static builds on macOS
because of a weird linking issue.

Run the following command to create a static binary for macOS:

```console
git clone --depth=1 https://github.com/dunglas/static-php-cli.git --branch=feat/embed 
cd static-php-cli
composer install --no-dev -a
./bin/spc doctor
./bin/spc fetch --with-php=8.2 -A
./bin/spc build --enable-zts --build-embed --debug "opcache"
export CGO_CFLAGS="$(./buildroot/bin/php-config --includes | sed s#-I/#-I$PWD/buildroot/#g)"
export CGO_LDFLAGS="-L$PWD/buildroot/lib $(./buildroot/bin/php-config --ldflags) $(./buildroot/bin/php-config --libs)"

git clone --depth=1 https://github.com/dunglas/frankenphp.git
cd frankenphp/caddy/frankenphp
go build -buildmode=pie -tags "cgo netgo osusergo static_build" -ldflags "-linkmode=external -extldflags -static-pie"
```
