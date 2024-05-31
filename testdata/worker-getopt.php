<?php

do {
    $ok = frankenphp_handle_request(function (): void {
        print_r($_SERVER);
    });

    getopt('abc');
} while ($ok);
