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
```Caddyfile
{
	frankenphp
}

# The domain name of your server
localhost

route {
	root * public/

	# Add trailing slash for directory requests
	@canonicalPath {
		file {path}/index.php
		not path */
	}
	redir @canonicalPath {path}/ 308

	# If the requested file does not exist, try index files
	@indexFiles file {
		try_files {path} {path}/index.php index.php
		split_path .php
	}
	rewrite @indexFiles {http.matchers.file.relative}

	# FrankenPHP!
	@phpFiles path *.php
	php @phpFiles
	encode zstd gzip
	file_server

	respond 404
}
```
3. Start FrankenPHP from the root directory of your Laravel project: `./frankenphp run`
