# Building Custom Docker Image

[FrankenPHP Docker images](https://hub.docker.com/repository/docker/dunglas/frankenphp) are based on [official PHP images](https://hub.docker.com/_/php/). Alpine Linux and Debian variants are provided for popular architectures.

## How to Use The Images

Create a `Dockerfile` in your project:

```Dockerfile
FROM dunglas/frankenphp
COPY . /app/public
CMD [ "php", "./your-script.php" ]
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
