<?php

$fail = random_int(1, 100) < 1;
$wait = random_int(1, 5);

sleep($wait);
if ($fail) {
    exit(1);
}

while (frankenphp_handle_request(function () {
    echo "ok";
})) {
    $fail = random_int(1, 100) < 10;
    if ($fail) {
        exit(1);
    }
}
