# Building Custom Docker Image

[FrankenPHP Docker images](https://hub.docker.com/repository/docker/dunglas/frankenphp) are based on [official PHP images](https://hub.docker.com/_/php/). Alpine Linux and Debian variants are provided for popular architectures.

## How to Use The Images

Create a `Dockerfile` in your project:

```Dockerfile
FROM dunglas/frankenphp

COPY . /app/public
```

Then, run the commands to build and run the Docker image:

```
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

```Dockerfile
FROM dunglas/frankenphp

# ...

ENV FRANKENPHP_CONFIG="worker ./public/index.php"
```

# Using a Volume in Development

To develop easily with FrankenPHP, mount the directory from your host containing the source code of the app as a volume in the Docker container:

```
docker run -v $PWD:/app/public -p 80:80 -p 443:443 my-php-app
```

With Docker Compose:

```yaml
# docker-compose.yml

version: '3.1'

services:

  php:
    image: dunglas/frankenphp
    # uncomment the following line if you want to use a custom Dockerfile
    #build: .
    restart: always
    ports:
      - 80:80
      - 443:443
    volumes:
      - ./:/app/public
```
