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
$ docker build -t my-php-app .
$ docker run -it --rm --name my-running-app my-php-app
```

## How to Install More PHP Extensions

The [`docker-php-extension-installer`](https://github.com/mlocati/docker-php-extension-installer) script is provided in the base image.
Adding additional PHP extensions is straightforwardd:

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

# Enabling the Worker Mode by Default

Set the `FRANKENPHP_CONFIG` environment variable to start FrankenPHP with a worker script:

```dockerfile
FROM dunglas/frankenphp

# ...

ENV FRANKENPHP_CONFIG="worker ./public/index.php"
```

# Using a Volume in Development

To develop easily with FrankenPHP, mount the directory from your host containing the source code of the app as a volume in the Docker container:

```console
docker run -v $PWD:/app/public -p 80:80 -p 443:443 my-php-app
```

With Docker Compose:

```yaml
# compose.yml

version: '3.1'

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
