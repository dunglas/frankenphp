# Efficiently Serving Large Static Files (`X-Sendfile`/`X-Accel-Redirect`)

Usually, static files can be served directly by the web server,
but sometimes it's necessary to execute some PHP code before sending them:
access control, statistics, custom HTTP headers...

Unfortunately, using PHP to serve large static files is inefficient compared to
direct use of the web server (memory overload, reduced performance...).

FrankenPHP lets you delegate the sending of static files to the web server
**after** executing customized PHP code.

To do this, your PHP application simply needs to define a custom HTTP header
containing the path of the file to be served. FrankenPHP takes care of the rest.

This feature is known as **`X-Sendfile`** for Apache, and **`X-Accel-Redirect`** for NGINX.

In the following examples, we assume that the document root of the project is the `public/` directory.
and that we want to use PHP to serve files stored outside the `public/` directory,
from a directory named `private-files/`.

## Configuration

First, add the following configuration to your `Caddyfile` to enable this feature:

```patch
    root * public/
    # ...

+	# Needed for Symfony, Laravel and other projects using the Symfony HttpFoundation component
+	request_header X-Sendfile-Type x-accel-redirect
+
+	intercept {
+		@accel header X-Accel-Redirect *
+		handle_response @accel {
+			root * private-files/
+			rewrite * {resp.header.X-Accel-Redirect}
+			method * GET
+
+			# Remove the X-Accel-Redirect header set by PHP for increased security
+			header -X-Accel-Redirect
+
+			file_server
+		}
+	}

	php_server
```


## Plain PHP

Set the relative file path (from `private-files/`) as the value of the `X-Accel-Redirect` header:

```php
header('X-Accel-Redirect: file.txt');
```

## Projects using the Symfony HttpFoundation component (Symfony, Laravel, Drupal...)

Symfony HttpFoundation [natively supports this feature](https://symfony.com/doc/current/components/http_foundation.html#serving-files).
It will automatically determine the correct value for the `X-Accel-Redirect` header and add it to the response.

```php
use Symfony\Component\HttpFoundation\BinaryFileResponse;

BinaryFileResponse::trustXSendfileTypeHeader();
$response = new BinaryFileResponse(__DIR__.'/../private-files/file.txt');

// ...
```
