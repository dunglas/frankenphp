# Building Custom Docker Image

[FrankenPHP Docker images](https://hub.docker.com/r/dunglas/frankenphp) are based on [official PHP images](https://hub.docker.com/_/php/). Alpine Linux and Debian variants are provided for popular architectures. Variants for PHP 8.2 and PHP 8.3 are provided. [Browse tags](https://hub.docker.com/repository/docker/dunglas/frankenphp).

## How to Use The Images

Create a `Dockerfile` in your project:

```dockerfile
FROM dunglas/frankenphp

COPY . /app/public
```

Then, run the commands to build and run the Docker image:

```console
docker build -t my-php-app .
docker run -it --rm --name my-running-app my-php-app
```

## How to Install More PHP Extensions

The [`docker-php-extension-installer`](https://github.com/mlocati/docker-php-extension-installer) script is provided in the base image.
Adding additional PHP extensions is straightforward:

```dockerfile
FROM dunglas/frankenphp

# add additional extensions here:
RUN install-php-extensions \
    pdo_mysql \
    gd \
    intl \
    zip \
    opcache

# ...
```

## How to Install More Caddy Modules

FrankenPHP is built on top of Caddy, and all [Caddy modules](https://caddyserver.com/docs/modules/) can be used with FrankenPHP.

The easiest way to install custom Caddy modules is to use [xcaddy](https://github.com/caddyserver/xcaddy):

```dockerfile
FROM dunglas/frankenphp:latest-builder AS builder

# Copy xcaddy in the builder image
COPY --from=caddy:builder /usr/bin/xcaddy /usr/bin/xcaddy

# CGO must be enabled to build FrankenPHP
ENV CGO_ENABLED=1 XCADDY_SETCAP=1 XCADDY_GO_BUILD_FLAGS="-ldflags '-w -s'"
RUN xcaddy build \
    --output /usr/local/bin/frankenphp \
    --with github.com/dunglas/frankenphp=./ \
    --with github.com/dunglas/frankenphp/caddy=./caddy/ \
    # Mercure and Vulcain are included in the official build, but feel free to remove them
    --with github.com/dunglas/mercure/caddy \
    --with github.com/dunglas/vulcain/caddy
    # Add extra Caddy modules here

FROM dunglas/frankenphp AS runner

# Replace the official binary by the one contained your custom modules
COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp
```

The `builder` image provided by FrankenPHP contains a compiled version of libphp.
[Builders images](https://hub.docker.com/r/dunglas/frankenphp/tags?name=builder) are provided for all versions of FrankenPHP and PHP, both for Alpine and Debian.

> [!TIP]
>
> If you're using Alpine Linux and Symfony,
> you may need to [increase the default stack size](compile.md#using-xcaddy).

## Enabling the Worker Mode by Default

Set the `FRANKENPHP_CONFIG` environment variable to start FrankenPHP with a worker script:

```dockerfile
FROM dunglas/frankenphp

# ...

ENV FRANKENPHP_CONFIG="worker ./public/index.php"
```

## Using a Volume in Development

To develop easily with FrankenPHP, mount the directory from your host containing the source code of the app as a volume in the Docker container:

```console
docker run -v $PWD:/app/public -p 80:80 -p 443:443 my-php-app
```

With Docker Compose:

```yaml
# compose.yaml

services:
  php:
    image: dunglas/frankenphp
    # uncomment the following line if you want to use a custom Dockerfile
    #build: .
    # uncomment the following line if you want to run this in a production environment
    # restart: always
    ports:
      - 80:80
      - 443:443
    volumes:
      - ./:/app/public
      - caddy_data:/data
      - caddy_config:/config
    # comment the following line in production, it allows to have nice human-readable logs in dev
    tty: true

# Volumes needed for Caddy certificates and configuration
volumes:
  caddy_data:
  caddy_config:
```
