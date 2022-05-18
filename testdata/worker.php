<?php

$i = 0;
do {
    $ok = frankenphp_handle_request(function () use ($i): void {
        echo sprintf("Requests handled: %d (request time: %s)\n", $i, $_SERVER['REQUEST_TIME_FLOAT']);
    
        var_export($_GET);
        var_export($_POST);
        var_export($_SERVER);    
    });

    $i++;
} while ($ok);
