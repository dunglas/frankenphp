<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd">

<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="en">
<head>
    <title>Test Page for FrankenPHP on AlmaLinux</title>
    <meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
    <style type="text/css">
        body {
            background-color: #FAF5F5;
            color: #000;
            font-size: 0.9em;
            font-family: sans-serif,helvetica;
            margin: 0;
            padding: 0;
        }
        :link, :visited {
            color: #0B2335;
        }
        a:hover {
            color: #0069DA;
        }
        h1 {
            text-align: center;
            margin: 0;
            padding: 0.6em 2em 0.4em;
            background-color: #0B2335;
            color: #fff;
            font-weight: normal;
            font-size: 1.75em;
            border-bottom: 2px solid #000;
        }
        h1 strong {
            font-weight: bold;
        }
        h2 {
            font-size: 1.1em;
            font-weight: bold;
        }
        hr {
            display: none;
        }
        .content {
            padding: 1em 5em;
        }
        .content-columns {
            position: relative;
            padding-top: 1em;
        }
        .content-column-left, .content-column-right {
            width: 47%;
            float: left;
            padding-bottom: 2em;
        }
        .content-column-left {
            padding-right: 3%;
        }
        .content-column-right {
            padding-left: 3%;
        }
        .logos {
            text-align: center;
            margin-top: 2em;
        }
        img {
            border: 2px solid #fff;
            padding: 2px;
            margin: 2px;
        }
        a:hover img {
            border: 2px solid #f50;
        }
        .footer {
            clear: both;
            text-align: center;
            font-size: xx-small;
        }
        .runtime-info {
            background: #efefef;
            padding: 0.5em;
            margin-top: 1em;
            font-size: 0.85em;
            border-left: 3px solid #0B2335;
        }
    </style>
</head>

<body>
    <h1>FrankenPHP <strong>Test Page</strong></h1>

    <div class="content">
        <div class="content-columns">
            <div class="content-column-left">
                <h2>If you are a visitor:</h2>

                <p>This default page means the site you tried to access is set up, but no specific content has been deployed yet.</p>

                <p>If you were expecting a different site, it's possible the owner hasn't uploaded any content, or they’re currently making updates.</p>

                <p>If you wish to notify the site administrator, try contacting "webmaster" at the domain you visited. For example: webmaster@example.com.</p>

                <p>Learn more about FrankenPHP at the <a href="https://frankenphp.dev/">official website</a>.</p>
                <hr />
            </div>

            <div class="content-column-right">
                <h2>If you are the administrator:</h2>

                <p>Your server is running and serving requests using FrankenPHP, integrated as a module within Caddy.</p>

                <p>To replace this page, simply deploy your application files to the configured web root directory in your Caddy setup. If you’re using PHP, FrankenPHP will handle it natively.</p>

                <p>Configuration is handled in your <code>Caddyfile</code>. Make sure your <code>root</code> and <code>php_server</code> directives are properly set for your site.</p>

                <div class="runtime-info">
                    <strong>Served by PHP SAPI: </strong> <?php echo php_sapi_name() ?><br />
                </div>

                <div class="logos">
                    <a href="https://frankenphp.dev/"><img src="assets/frankenphp.svg" height="50" width="166" alt="[ Powered by FrankenPHP ]" /></a>
                    <a href="https://caddyserver.com/"><img src="assets/caddy.png" height="50" width="166" alt="[ Powered by Caddy ]" /></a>
                </div>
            </div>
        </div>
    </div>
    <div class="footer">
        <a href="https://frankenphp.dev">FrankenPHP</a> is an open-source web server module for PHP built on top of <a href="https://caddyserver.com">Caddy</a>.
    </div>
</body>
</html>
