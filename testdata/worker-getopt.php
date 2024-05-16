<?php

$i = 0;
do {
    $ok = frankenphp_handle_request(function () use ($i): void {
        echo sprintf("Requests handled: %d\n", $i);
    });

    $i++;

    getopt('abc');
} while ($ok);
