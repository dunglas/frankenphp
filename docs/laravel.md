# Laravel

## Docker

Serving a [Laravel](https://laravel.com) web application with FrankenPHP is as easy as mounting the project in the `/app` directory of the official Docker image.

Run this command from the main directory of your Laravel app:

```console
docker run -p 443:443 -v $PWD:/app dunglas/frankenphp
```

And enjoy!

## Local Installation

Alternatively, you can run your Laravel projects with FrankenPHP from your local machine:

1. [Download the binary corresponding to your system](https://github.com/dunglas/frankenphp/releases)
2. Add the following configuration to a file named `Caddyfile` in the root directory of your Laravel project:
```caddyfile
{
	frankenphp
	order php_server before file_server
}

# The domain name of your server
localhost {
    # Enable compression (optional)
    encode zstd gzip
    # Execute PHP files in the current directory and serve assets
    php_server
}
```
3. Start FrankenPHP from the root directory of your Laravel project: `./frankenphp run`

## Laravel Octane

See [this Pull Request](https://github.com/laravel/octane/pull/764).
