<?php

$fail = random_int(1, 100) < 10;
$wait = random_int(1000 * 100, 1000 * 500); // wait 100ms - 500ms

usleep($wait);
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
