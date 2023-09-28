# Laravel

Serving a [Laravel](https://laravel.com) web application with FrankenPHP is as easy as mounting the project in the `/app` directory of the official Docker image.

Run this command from the main directory of your Laravel app:

```console
docker run -p 443:443 -v $PWD:/app dunglas/frankenphp
```

And enjoy!
