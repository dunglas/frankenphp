<?php

$fn = static function () {
    echo sprintf("Counter (%s)", $_GET['i'] ?? 'i not set');
};

$loopMax = $_SERVER['EVERY'] ?? 10;
$loops = 0;
do {
    $ret = \frankenphp_handle_request($fn);
} while ($ret && (-1 === $loopMax || ++$loops < $loopMax));

exit(0);
