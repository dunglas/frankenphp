<?php

do {
    $ok = frankenphp_handle_request(function () use ($i): void {
        echo 'Hello, world';
    });

    die('Hello');
} while ($ok);
