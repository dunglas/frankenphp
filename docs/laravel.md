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
    	# Set the webroot to the public/ dir
    	root * public/
    	# Enable compression (optional)
    	encode zstd gzip
    	# Execute PHP files in the current directory and serve assets
    	php_server {
    		# Required for the public/storage/ dir
    		resolve_root_symlink
    	}
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

//Additional Parameter
{--server=frankenphp : The server that should be used to serve the application}
{--host=127.0.0.1 : The IP address the server should bind to}
{--port= : The port the server should be available on [default: "8000"]}
{--workers=auto : The number of workers that should be available to handle requests}
{--task-workers=auto : The number of task workers that should be available to handle tasks}
{--max-requests=500 : The number of requests to process before reloading the server}
{--caddyfile= : The path to the FrankenPHP Caddyfile file}
{--https : Enable HTTPS, HTTP/2, and HTTP/3, and automatically generate and renew certificates}
{--watch : Automatically reload the server when the application is modified}
{--poll : Use file system polling while watching in order to watch files over a network}
{--log-level= : Log messages at or above the specified log level}';
```

Learn more about [Laravel Octane in its official documentation](https://laravel.com/docs/octane).

