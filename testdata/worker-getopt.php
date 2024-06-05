<?php

do {
    $ok = frankenphp_handle_request(function (): void {
        print_r($_SERVER);
    });

    getopt('abc');

    if (!isset($_SERVER['HTTP_REQUEST'])) {
        exit(1);
    }
    if (isset($_SERVER['FRANKENPHP_WORKER'])) {
        exit(2);
    }
    if (isset($_SERVER['FOO'])) {
        exit(3);
    }
} while ($ok);
