<?php

file_put_contents(__DIR__ . '/log', 'before: '.print_r($_SERVER, true), FILE_APPEND);

$i = 0;
do {
    $ok = frankenphp_handle_request(function () use ($i): void {
        echo sprintf("Requests handled: %d\n", $i);
        file_put_contents(__DIR__ . '/log', 'inside: '.print_r($_SERVER, true), FILE_APPEND);
    });

    $i++;

    getopt('abc');
    if ($ok)
        file_put_contents(__DIR__ . '/log', 'after: '.print_r($_SERVER, true), FILE_APPEND);
} while ($ok);
