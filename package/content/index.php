<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Test Page for FrankenPHP</title>
    <style>
        body {
            background-color: #FAF5F5;
            color: #000;
            font-size: 0.9em;
            font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            margin: 0;
            padding: 0;
            line-height: 1.6;
        }
        a {
            color: #0B2335;
            text-decoration: none;
        }
        a:hover {
            color: #0069DA;
            text-decoration: underline;
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
        .content {
            padding: 1em 5em;
            max-width: 1200px;
            margin: 0 auto;
        }
        .content-columns {
            display: flex;
            flex-wrap: wrap;
            gap: 2em;
            padding-top: 1em;
        }
        .content-column-left, .content-column-right {
            flex: 1;
            min-width: 300px;
        }
        .logos {
            text-align: center;
            margin-top: 2em;
            display: flex;
            justify-content: center;
            gap: 1em;
            flex-wrap: wrap;
        }
        img {
            border: 2px solid #fff;
            padding: 2px;
            margin: 2px;
            max-width: 100%;
            height: auto;
        }
        a:hover img {
            border: 2px solid #f50;
        }
        .footer {
            text-align: center;
            font-size: 0.8em;
            padding: 1em;
            margin-top: 2em;
            border-top: 1px solid #eee;
        }
        .runtime-info {
            background: #efefef;
            padding: 0.8em;
            margin-top: 1em;
            font-size: 0.85em;
            border-left: 3px solid #0B2335;
            border-radius: 0 4px 4px 0;
        }

        /* Responsive design */
        @media (max-width: 768px) {
            .content {
                padding: 1em 2em;
            }
            .content-columns {
                flex-direction: column;
            }
            h1 {
                padding: 0.6em 1em 0.4em;
                font-size: 1.5em;
            }
        }
    </style>
</head>

<body>
    <header>
        <h1>FrankenPHP <strong>Test Page</strong></h1>
    </header>

    <main class="content">
        <div class="content-columns">
            <section class="content-column-left">
                <h2>If you are a member of the general public:</h2>

                <p>The fact that you are seeing this page indicates that the website you just visited is either experiencing problems, or is undergoing routine maintenance.</p>

                <p>
                    If you would like to let the administrators of this website know that you've seen this page instead of the page you expected, you should send them e-mail.
                    In general, mail sent to the name "webmaster" and directed to the website's domain should reach the appropriate person.
                </p>

                <p>For example, try contacting <a href="mailto:webmaster@<?php echo $_SERVER['SERVER_NAME'] ?? 'example.com'; ?>">webmaster@<?php echo $_SERVER['SERVER_NAME'] ?? 'example.com'; ?></a>.</p>

                <p>Learn more about FrankenPHP at the <a href="https://frankenphp.dev/">official website</a>.</p>
            </section>

            <section class="content-column-right">
                <h2>If you are the website administrator:</h2>

                <p>Your server is running and serving requests using FrankenPHP, powered by Caddy</p>

                <p>To replace this page, deploy your application files to <code><?php echo getcwd(); ?></code>.</p>

                <p>Configuration is handled in your <code>Caddyfile</code>.</p>

                <div class="runtime-info">
                    <strong>Served by PHP SAPI: </strong> <?php echo php_sapi_name() ?><br />
                </div>

                <div class="logos">
                    <a href="https://frankenphp.dev/"><img src="assets/frankenphp.svg" height="50" width="166" alt="Powered by FrankenPHP" /></a>
                    <a href="https://caddyserver.com/"><img src="assets/caddy.png" height="50" width="166" alt="Powered by Caddy" /></a>
                </div>
            </section>
        </div>
    </main>

    <footer class="footer">
        <p><a href="https://frankenphp.dev">FrankenPHP</a> is an open-source web server for PHP built on top of <a href="https://caddyserver.com">Caddy</a>.</p>
    </footer>
</body>
</html>