<?php

$fn = require $_SERVER['SCRIPT_FILENAME'];
if (!isset($_SERVER['FRANKENPHP_WORKER'])) {
    $fn();
    return;
}

while (frankenphp_handle_request($fn)) {}
