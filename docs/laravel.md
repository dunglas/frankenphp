# Laravel

## Docker

Serving a [Laravel](https://laravel.com) web application with FrankenPHP is as easy as mounting the project in the `/app` directory of the official Docker image.

Run this command from the main directory of your Laravel app:

```console
docker run -p 80:80 -p 443:443 -p 443:443/udp -v $PWD:/app dunglas/frankenphp
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
    	# Set the webroot to the public/ directory
    	root * public/
    	# Enable compression (optional)
    	encode zstd br gzip
    	# Execute PHP files from the public/ directory and serve assets
    	php_server
    }
    ```

3. Start FrankenPHP from the root directory of your Laravel project: `./frankenphp run`

## Laravel Octane

Octane may be installed via the Composer package manager:

```console
composer require laravel/octane
```

After installing Octane, you may execute the `octane:install` Artisan command, which will install Octane's configuration file into your application:

```console
php artisan octane:install --server=frankenphp
```

The Octane server can be started via the `octane:start` Artisan command.

```console
php artisan octane:start
```

The `octane:start` command can take the following options:

* `--host`: The IP address the server should bind to (default: `127.0.0.1`)
* `--port`: The port the server should be available on (default: `8000`)
* `--admin-port`: The port the admin server should be available on (default: `2019`)
* `--workers`: The number of workers that should be available to handle requests (default: `auto`)
* `--max-requests`: The number of requests to process before reloading the server (default: `500`)
* `--caddyfile`: The path to the FrankenPHP `Caddyfile` file
* `--https`: Enable HTTPS, HTTP/2, and HTTP/3, and automatically generate and renew certificates
* `--http-redirect`: Enable HTTP to HTTPS redirection (only enabled if --https is passed)
* `--watch`: Automatically reload the server when the application is modified
* `--poll`: Use file system polling while watching in order to watch files over a network
* `--log-level`: Log messages at or above the specified log level

Learn more about [Laravel Octane in its official documentation](https://laravel.com/docs/octane).
