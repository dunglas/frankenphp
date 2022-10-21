<?php

do {
    $ok = frankenphp_handle_request(function (): void {
        echo 'Hello, world';
    });

    die('Hello');
} while ($ok);
