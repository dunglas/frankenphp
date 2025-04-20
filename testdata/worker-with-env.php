<?php

$i = 0;
$env = $_SERVER['APP_ENV'];
do {
    $ok = frankenphp_handle_request(function () use ($i, $env): void {
        echo "Worker has APP_ENV=$env";
    });

    $i++;
} while ($ok);
