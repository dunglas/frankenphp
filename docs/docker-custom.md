# Building Custom Docker Image

Create a `Dockerfile` FROM `dunglas/frankenphp` and add additional php extensions:

```dockerfile
FROM dunglas/frankenphp:latest

# add additional extensions here:
RUN install-php-extensions \
    pdo_mysql \
    gd \
    intl \
    zip \
    opcache
```

Build the custom FrankenPHP image:

```bash
docker build -t custom-franken-php .
```

Run your application with created custom FrankenPHP image:

```bash
docker run -e FRANKENPHP_CONFIG="worker ./public/index.php" -v $PWD:/app -p 80:80 -p 443:443 custom-franken-php
```
