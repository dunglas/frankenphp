<?php

//echo 'Worker started...'.PHP_EOL;

while (frankenphp_handle_request()) {
    echo 'Handling request...'.PHP_EOL;

    include 'super-globals.php';
}

//echo 'Worker finishing...'.PHP_EOL;
