<?php

$env = $_SERVER['APP_ENV'] ?? '';
do {
    $ok = frankenphp_handle_request(function () use ($env): void {
        echo "Worker has APP_ENV=$env";
    });
} while ($ok);
